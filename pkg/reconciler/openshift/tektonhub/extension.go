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
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	console "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	openshiftCommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	hubprefix                  string = "tekton-hub"
	tektonHubAPIResourceKey    string = "api"
	tektonHubUiResourceKey     string = "ui"
	CreatedByValue             string = "TektonHub"
	ConsoleHubLinkInstallerSet        = "ConsoleHubLink"
)

var replaceVal = map[string]string{
	"POSTGRES_DB":       "POSTGRESQL_DATABASE",
	"POSTGRES_USER":     "POSTGRESQL_USER",
	"POSTGRES_PASSWORD": "POSTGRESQL_PASSWORD",
}

var (
	db  string = fmt.Sprintf("%s-%s", hubprefix, "db")
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
	return []mf.Transformer{
		UpdateDbDeployment(),
		openshiftCommon.RemoveRunAsUser(),
		openshiftCommon.RemoveRunAsUserForJob(),
		openshiftCommon.RemoveFsGroupForDeployment(),
		openshiftCommon.RemoveFsGroupForJob(),
	}
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

	// Set Auth Url in Tekton Hub Status
	if err := oe.SetAuthBaseURL(ctx, th, apiRouteManifest); err != nil {
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
	th := tc.(*v1alpha1.TektonHub)
	consoleCLILS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: ConsoleHubLinkInstallerSet,
		},
	}

	consoleHubLinkLabelSelector, err := common.LabelSelector(consoleCLILS)
	if err != nil {
		return err
	}

	exist, err := checkIfInstallerSetExist(ctx, oe.operatorClientSet, common.TargetVersion(th), consoleHubLinkLabelSelector)
	if err != nil {
		return err
	}

	if !exist {
		hubConsoleLinkManifest := oe.manifest.Append()
		if err := applyHubConsoleLinkManifest(&hubConsoleLinkManifest); err != nil {
			return err
		}

		if err := consoleLinkTransform(ctx, &hubConsoleLinkManifest, th.Status.GetUiRoute()); err != nil {
			return err
		}

		if err := createInstallerSet(ctx, oe.operatorClientSet, th, hubConsoleLinkManifest,
			common.TargetVersion(th), ConsoleHubLinkInstallerSet, "console-link-hub"); err != nil {
			return err
		}
	}

	return nil
}

func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	labelSelector string) (bool, error) {

	installerSets, err := oc.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return false, err
	}

	if len(installerSets.Items) == 0 {
		return false, nil
	}

	if len(installerSets.Items) == 1 {
		// if already created then check which version it is
		version, ok := installerSets.Items[0].Labels[v1alpha1.ReleaseVersionKey]
		if ok && version == relVersion {
			// if installer set already exist and release version is same
			// then ignore and move on
			return true, nil
		}
	}

	// release version doesn't exist or is different from expected
	// deleted existing InstallerSet and create a new one
	// or there is more than one installerset (unexpected)
	if err = oc.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		return false, err
	}

	return false, v1alpha1.RECONCILE_AGAIN_ERR
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, ta *v1alpha1.TektonHub,
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix string) error {

	is := makeInstallerSet(ta, manifest, installerSetPrefix, releaseVersion, component)

	if _, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func makeInstallerSet(ta *v1alpha1.TektonHub, manifest mf.Manifest, prefix, releaseVersion, component string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(ta, ta.GetGroupVersionKind())
	labels := map[string]string{
		v1alpha1.CreatedByKey:      CreatedByValue,
		v1alpha1.InstallerSetType:  component,
		v1alpha1.ReleaseVersionKey: releaseVersion,
	}
	namePrefix := fmt.Sprintf("%s-", prefix)

	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix,
			Labels:       labels,
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: ta.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func applyHubConsoleLinkManifest(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	location := filepath.Join(koDataDir, "openshift", "tekton-hub")
	return common.AppendManifest(manifest, location)
}

func consoleLinkTransform(ctx context.Context, manifest *mf.Manifest, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("route url should not be empty")
	}
	logger := logging.FromContext(ctx)
	logger.Debug("Transforming manifest")

	transformers := []mf.Transformer{
		replaceURLConsoleLink(baseURL),
	}

	transformManifest, err := manifest.Transform(transformers...)
	if err != nil {
		return err
	}

	*manifest = transformManifest
	return nil
}

func replaceURLConsoleLink(baseURL string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConsoleLink" {
			return nil
		}
		cl := &console.ConsoleLink{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cl)
		if err != nil {
			return err
		}

		cl.Spec.Href = baseURL

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cl)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
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

		if d.Name == db {
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

func (oe openshiftExtension) SetAuthBaseURL(ctx context.Context, th *v1alpha1.TektonHub, apiRouteManifest mf.Manifest) error {
	// Get the api secret
	secret, err := oe.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Get(ctx, "tekton-hub-api", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			th.Status.SetAuthRoute("")
		} else {
			return err
		}
	}

	if len(secret.Data["GH_CLIENT_ID"]) != 0 || len(secret.Data["GL_CLIENT_ID"]) != 0 || len(secret.Data["BB_CLIENT_ID"]) != 0 {
		// Get the host of Auth route
		authRoute, err := getRouteHost(&apiRouteManifest, "tekton-hub-auth")
		if err != nil {
			return err
		}
		th.Status.SetAuthRoute(fmt.Sprintf("https://%s", authRoute))

		if secret.Data == nil || string(secret.Data["AUTH_BASE_URL"]) != th.Status.AuthRouteUrl {

			if secret.StringData == nil {
				secret.StringData = make(map[string]string)
			}

			secret.StringData["AUTH_BASE_URL"] = th.Status.AuthRouteUrl

			_, err = oe.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Update(ctx, secret, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	} else {
		th.Status.SetAuthRoute("")
	}

	return nil
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
