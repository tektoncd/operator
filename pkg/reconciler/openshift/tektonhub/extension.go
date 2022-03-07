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

package tektonhub

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	hubprefix               string = "tekton-hub"
	tektonHubAPIResourceKey string = "api"
	tektonHubUiResourceKey  string = "ui"
)

var replaceVal = map[string]string{
	"POSTGRES_DB":       "POSTGRESQL_DATABASE",
	"POSTGRES_USER":     "POSTGRESQL_USER",
	"POSTGRES_PASSWORD": "POSTGRESQL_PASSWORD",
}

var (
	api string = fmt.Sprintf("%s-%s", hubprefix, "api")
	ui  string = fmt.Sprintf("%s-%s", hubprefix, "ui")
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		kubeClientSet:     kubeclient.Get(ctx),
		manifest:          manifest,
	}
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
	manifest          mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{UpdateDbDeployment()}
}

func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	th := tc.(*v1alpha1.TektonHub)
	logger := logging.FromContext(ctx)
	targetNs := th.Spec.GetTargetNamespace()
	hubDir := filepath.Join(common.ComponentDir(th), common.TargetVersion(th), tektonHubAPIResourceKey)
	manifest := oe.manifest.Append()

	if err := common.AppendManifest(&manifest, hubDir); err != nil {
		return err
	}

	apiRouteManifest := manifest.Filter(mf.ByKind("Route"))
	apiRouteManifest, err := apiRouteManifest.Transform(
		mf.InjectOwner(th),
		mf.InjectNamespace(targetNs),
	)
	if err != nil {
		logger.Error("failed to transform manifest")
		return err
	}
	if err := apiRouteManifest.Apply(); err != nil {
		return err
	}

	// Get the host of API route
	apiRoute, err := getRouteHost(&apiRouteManifest, api)
	if err != nil {
		return err
	}
	th.Status.SetApiRoute(fmt.Sprintf("https://%s", apiRoute))

	// Get the host of Auth route
	authRoute, err := getRouteHost(&apiRouteManifest, "tekton-hub-auth")
	if err != nil {
		return err
	}
	th.Status.SetAuthRoute(fmt.Sprintf("https://%s", authRoute))

	// Update the secrets of API with the Auth Route value
	if err := oe.updateApiSecret(ctx, th, authRoute, targetNs); err != nil {
		return err
	}

	// Create UI route based on the value of ui i.e. false/true

	uiHubDir := filepath.Join(common.ComponentDir(th), common.TargetVersion(th), tektonHubUiResourceKey)
	uiManifest := oe.manifest.Append()

	if err := common.AppendManifest(&uiManifest, uiHubDir); err != nil {
		return err
	}

	uiRouteManifest := uiManifest.Filter(mf.ByKind("Route"))
	uiRouteManifest, err = uiRouteManifest.Transform(
		mf.InjectOwner(th),
		mf.InjectNamespace(targetNs),
	)
	if err != nil {
		logger.Error("failed to transform manifest")
		return err
	}
	if err := uiRouteManifest.Apply(); err != nil {
		return err
	}

	uiRoute, err := getRouteHost(&uiRouteManifest, ui)
	if err != nil {
		return err
	}

	th.Status.SetUiRoute(fmt.Sprintf("https://%s", uiRoute))

	return nil
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {

	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

// Updates the AUTH_BASE_URL in the API secret with the Auth Route value
func (oe openshiftExtension) updateApiSecret(ctx context.Context, th *v1alpha1.TektonHub, authRoute, namespace string) error {
	secret, err := oe.kubeClientSet.CoreV1().Secrets(namespace).Get(ctx, api, metav1.GetOptions{})
	if err != nil {
		th.Status.MarkApiDependencyMissing(fmt.Sprintf("API secret is not present %v", err.Error()))
		return err
	}

	if secret.Data["AUTH_BASE_URL"] != nil && len(secret.Data["AUTH_BASE_URL"]) != 0 {
		delete(secret.Data, "AUTH_BASE_URL")
	}

	secret.StringData = make(map[string]string)
	secret.StringData["AUTH_BASE_URL"] = fmt.Sprintf("https://%s", authRoute)

	_, err = oe.kubeClientSet.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Get the Host value of the Route created
func getRouteHost(manifest *mf.Manifest, routeName string) (string, error) {
	var hostUrl string
	for _, r := range manifest.Filter(mf.ByKind("Route")).Resources() {
		u, err := manifest.Client.Get(&r)
		if err != nil {
			return "", err
		}
		if u.GetName() == routeName {
			route := &routev1.Route{}
			if err := scheme.Scheme.Convert(u, route, nil); err != nil {
				return "", err
			}
			hostUrl = route.Spec.Host
		}
	}
	return hostUrl, nil
}

func UpdateDbDeployment() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		if d.Name == "db" {
			env := d.Spec.Template.Spec.Containers[0].Env

			replaceEnv(env)

			d.Spec.Template.Spec.Containers[0].Env = env

			mountPath := "/var/lib/pgsql/data"
			d.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = mountPath

			replaceProbeCommand(d.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec.Command)
			replaceProbeCommand(d.Spec.Template.Spec.Containers[0].LivenessProbe.Exec.Command)

			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
			if err != nil {
				return err
			}
			u.SetUnstructuredContent(unstrObj)

			return nil
		}

		return nil
	}
}

func replaceProbeCommand(data []string) {
	if strings.Contains(data[2], "POSTGRES_USER") {
		data[2] = strings.ReplaceAll(data[2], "POSTGRES_USER", "POSTGRESQL_USER")
	}
	if strings.Contains(data[2], "POSTGRES_DB") {
		data[2] = strings.ReplaceAll(data[2], "POSTGRES_DB", "POSTGRESQL_DATABASE")
	}
}

func replaceEnv(envs []corev1.EnvVar) {
	for i, e := range envs {
		_, ok := replaceVal[e.Name]
		if ok {
			envs[i].Name = replaceVal[e.Name]
		}
	}
}
