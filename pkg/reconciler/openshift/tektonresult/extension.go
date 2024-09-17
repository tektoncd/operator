/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tektonresult

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
)

const (
	// manifests console plugin yaml directory location
	routeRBACYamlDirectory  = "static/tekton-results/route-rbac"
	internalDBYamlDirectory = "static/tekton-results/internal-db"
	logsRBACYamlDirectory   = "static/tekton-results/logs-rbac"
	deploymentAPI           = "tekton-results-api"
	apiContainerName        = "api"
	boundSAVolume           = "bound-sa-token"
	boundSAPath             = "/var/run/secrets/openshift/serviceaccount"
	lokiStackTLSCAEnvVar    = "LOGGING_PLUGIN_CA_CERT"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	version := os.Getenv(v1alpha1.VersionEnvKey)
	if version == "" {
		logger.Fatal("Failed to find version from env")
	}

	routeManifest, err := getRouteManifest()
	if err != nil {
		logger.Fatalf("Failed to fetch route rbac static manifest: %v", err)
	}

	internalDBManifest, err := getDBManifest()
	if err != nil {
		logger.Fatalf("Failed to fetch internal db static manifest: %v", err)
	}

	logsRBACManifest, err := getloggingRBACManifest()
	if err != nil {
		logger.Fatalf("Failed to fetch logs RBAC manifest: %v", err)
	}

	ext := openshiftExtension{
		installerSetClient: client.NewInstallerSetClient(operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets(),
			version, "results-ext", v1alpha1.KindTektonResult, nil),
		internalDBManifest: internalDBManifest,
		routeManifest:      routeManifest,
		logsRBACManifest:   logsRBACManifest,
	}
	return ext
}

type openshiftExtension struct {
	installerSetClient *client.InstallerSetClient
	routeManifest      *mf.Manifest
	internalDBManifest *mf.Manifest
	logsRBACManifest   *mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	instance := comp.(*v1alpha1.TektonResult)

	return []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
		occommon.ApplyCABundles,
		injectBoundSAToken(instance.Spec.ResultsAPIProperties),
		injectLokiStackTLSCACert(instance.Spec.LokiStackProperties),
	}
}

func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	result := tc.(*v1alpha1.TektonResult)

	mf := mf.Manifest{}
	if !result.Spec.IsExternalDB {
		mf = *oe.internalDBManifest
	}

	if (result.Spec.LokiStackName != "" && result.Spec.LokiStackNamespace != "") ||
		strings.EqualFold(result.Spec.LogsType, "LOKI") {
		mf = mf.Append(*oe.logsRBACManifest)
	}

	return oe.installerSetClient.PreSet(ctx, tc, &mf, filterAndTransform())
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	mf := *oe.routeManifest
	return oe.installerSetClient.PostSet(ctx, tc, &mf, filterAndTransform())
}

func (oe openshiftExtension) Finalize(ctx context.Context, tc v1alpha1.TektonComponent) error {
	if err := oe.installerSetClient.CleanupPostSet(ctx); err != nil {
		return err
	}
	if err := oe.installerSetClient.CleanupPreSet(ctx); err != nil {
		return err
	}
	return nil
}

func getRouteManifest() (*mf.Manifest, error) {
	manifest := &mf.Manifest{}
	resultsRbac := filepath.Join(common.ComponentBaseDir(), routeRBACYamlDirectory)
	if err := common.AppendManifest(manifest, resultsRbac); err != nil {
		return nil, err
	}
	return manifest, nil
}

func getDBManifest() (*mf.Manifest, error) {
	manifest := &mf.Manifest{}
	internalDB := filepath.Join(common.ComponentBaseDir(), internalDBYamlDirectory)
	if err := common.AppendManifest(manifest, internalDB); err != nil {
		return nil, err
	}
	return manifest, nil
}

// function to add fine grained access control to results api if results config specifies that
// pipeline logs are managed by OpenShift Logging with OpenShift logging and OpenShift loki operators
func getloggingRBACManifest() (*mf.Manifest, error) {
	manifest := &mf.Manifest{}
	logsRbac := filepath.Join(common.ComponentBaseDir(), logsRBACYamlDirectory)
	if err := common.AppendManifest(manifest, logsRbac); err != nil {
		return nil, err
	}
	return manifest, nil
}

func filterAndTransform() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		resultImgs := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ResultsImagePrefix))

		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdResults),
			common.ApplyProxySettings,
			common.AddStatefulSetRestrictedPSA(),
			common.DeploymentImages(resultImgs),
			common.StatefulSetImages(resultImgs),
		}

		if err := common.Transform(ctx, manifest, comp, extra...); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

// injectBoundSAToken adds a sa token projected volume to the Results Deployment
func injectBoundSAToken(props v1alpha1.ResultsAPIProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if props.LogsAPI == nil || !*props.LogsAPI ||
			u.GetKind() != "Deployment" || u.GetName() != deploymentAPI {
			return nil
		}

		d := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		// find the matching container and add env and secret name object
		for i, container := range d.Spec.Template.Spec.Containers {
			if container.Name != apiContainerName {
				continue
			}
			add := true
			vol := corev1.Volume{
				Name: boundSAVolume,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
									Audience: "openshift",
									Path:     "token",
								},
							},
						},
					},
				},
			}
			for k := 0; k < len(d.Spec.Template.Spec.Volumes); k++ {
				if d.Spec.Template.Spec.Volumes[k].Name == boundSAVolume {
					d.Spec.Template.Spec.Volumes[k] = vol
					add = false
				}
			}
			if add {
				d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, vol)
			}

			volMount := corev1.VolumeMount{
				Name:      boundSAVolume,
				MountPath: boundSAPath,
			}

			add = true
			for k := 0; k < len(d.Spec.Template.Spec.Containers[i].VolumeMounts); k++ {
				if d.Spec.Template.Spec.Containers[i].VolumeMounts[k].Name == boundSAVolume {
					d.Spec.Template.Spec.Containers[i].VolumeMounts[k] = volMount
					add = false
				}
			}
			if add {
				d.Spec.Template.Spec.Containers[i].VolumeMounts = append(
					d.Spec.Template.Spec.Containers[i].VolumeMounts, volMount)
			}

			break
		}

		uObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(uObj)
		return nil
	}
}

// injectLokiStackTLSCACert adds a tls ca cert environment variable to the Results Deployment
// If the env variable already exists, it will be overwritten
func injectLokiStackTLSCACert(prop v1alpha1.LokiStackProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if prop.LokiStackNamespace == "" || prop.LokiStackName == "" ||
			u.GetKind() != "Deployment" || u.GetName() != deploymentAPI {
			return nil
		}

		d := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		// find the matching container and add env and secret name object
		for i, container := range d.Spec.Template.Spec.Containers {
			if container.Name != apiContainerName {
				continue
			}
			add := true
			env := corev1.EnvVar{
				Name: lokiStackTLSCAEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "openshift-service-ca.crt",
						},
						Key: "service-ca.crt",
					},
				},
			}

			// Check if the env variable already exists in the container
			// If it does, overwrite it
			for k := 0; k < len(d.Spec.Template.Spec.Containers[i].Env); k++ {
				if d.Spec.Template.Spec.Containers[i].Env[k].Name == lokiStackTLSCAEnvVar {
					d.Spec.Template.Spec.Containers[i].Env[k] = env
					add = false
				}
			}

			// If it doesn't exist, add it
			if add {
				d.Spec.Template.Spec.Containers[i].Env = append(
					d.Spec.Template.Spec.Containers[i].Env, env)
			}

			break
		}

		uObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(uObj)
		return nil
	}
}
