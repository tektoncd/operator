/*
Copyright 2024 The Tekton Authors

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

package openshiftpipelinesascode

import (
	"context"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestFilterAdditionalControllerManifest(t *testing.T) {
	testData := path.Join("testdata", "test-filter-manifest.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	filtered := filterAdditionalControllerManifest(manifest)
	assert.DeepEqual(t, len(filtered.Resources()), 5)

	routes := filtered.Filter(mf.All(mf.ByKind("Route")))
	assert.DeepEqual(t, len(routes.Resources()), 1)

	sms := filtered.Filter(mf.All(mf.ByKind("ServiceMonitor")))
	assert.DeepEqual(t, len(sms.Resources()), 1)
}

func TestUpdateAdditionControllerDeployment(t *testing.T) {
	testData := path.Join("testdata", "test-filter-manifest.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Deployment")))

	additionalPACConfig := v1alpha1.AdditionalPACControllerConfig{
		ConfigMapName: "test-configmap",
		SecretName:    "test-secret",
	}
	updatedDeployment, err := manifest.Transform(updateAdditionControllerDeployment(additionalPACConfig, "test"))
	assert.NilError(t, err)
	assert.DeepEqual(t, updatedDeployment.Resources()[0].GetName(), "test-pac-controller")

	expectedData := path.Join("testdata", "test-expected-additional-pac-dep.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(expectedData))
	assert.NilError(t, err)

	expected := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		assert.NilError(t, err)
	}

	got := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(updatedDeployment.Resources()[0].Object, got)
	if err != nil {
		assert.NilError(t, err)
	}

	if d := cmp.Diff(got, expected); d != "" {
		t.Errorf("failed to update additional pac controller deployment %s", diff.PrintWantGot(d))
	}

}

func TestUpdateAdditionControllerService(t *testing.T) {
	testData := path.Join("testdata", "test-filter-manifest.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Service")))

	updatedManifest, err := manifest.Transform(updateAdditionControllerService("test"))
	assert.NilError(t, err)
	assert.DeepEqual(t, updatedManifest.Resources()[0].GetName(), "test-pac-controller")
}

func TestUpdateAdditionControllerRoute(t *testing.T) {
	testData := path.Join("testdata", "test-filter-manifest.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Route")))

	updatedManifest, err := manifest.Transform(updateAdditionControllerRoute("test"))
	if err != nil {
		assert.NilError(t, err)
	}

	route := &routev1.Route{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(updatedManifest.Resources()[0].Object, route)
	if err != nil {
		assert.NilError(t, err)
	}
	expectedData := path.Join("testdata", "test-expected-additional-pac-route.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(expectedData))
	assert.NilError(t, err)

	expectedRoute := &routev1.Route{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expectedRoute)
	if err != nil {
		assert.NilError(t, err)
	}

	if d := cmp.Diff(route, expectedRoute); d != "" {
		t.Errorf("failed to update additional pac controller route %s", diff.PrintWantGot(d))
	}

}

func TestUpdateAdditionControllerServiceMonitor(t *testing.T) {
	testData := path.Join("testdata", "test-filter-manifest.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller-monitor"), mf.ByKind("ServiceMonitor")))

	updatedManifest, err := manifest.Transform(updateAdditionControllerServiceMonitor("test"))
	assert.NilError(t, err)
	assert.DeepEqual(t, updatedManifest.Resources()[0].GetName(), "test-pac-controller")
}

func TestUpdateAdditionControllerConfigMapWithDefaultCM(t *testing.T) {
	testData := path.Join("testdata", "test-filter-manifest.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code"), mf.ByKind("ConfigMap")))

	additionalPACConfig := v1alpha1.AdditionalPACControllerConfig{
		ConfigMapName: "pipelines-as-code",
		SecretName:    "test-secret",
	}
	updatedManifest, err := manifest.Transform(updateAdditionControllerConfigMap(additionalPACConfig))
	assert.NilError(t, err)
	assert.DeepEqual(t, updatedManifest.Resources()[0].GetName(), "pipelines-as-code")
}

func TestUpdateAdditionControllerConfigMap(t *testing.T) {
	testData := path.Join("testdata", "test-additional-pac-cm.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	additionalPACConfig := v1alpha1.AdditionalPACControllerConfig{
		ConfigMapName: "test-config",
		SecretName:    "test-secret",
		Settings:      map[string]string{"application-name": "Test CI application", "hub-url": "https://custom-hub-catalog.com"},
	}

	updatedManifest, err := manifest.Transform(updateAdditionControllerConfigMap(additionalPACConfig))
	if err != nil {
		assert.NilError(t, err)
	}

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(updatedManifest.Resources()[0].Object, cm)
	if err != nil {
		assert.NilError(t, err)
	}

	expectedTestData := path.Join("testdata", "test-expected-additional-pac-cm.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(expectedTestData))
	assert.NilError(t, err)
	expectedCM := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expectedCM)
	if err != nil {
		assert.NilError(t, err)
	}

	assert.NilError(t, err)

	if d := cmp.Diff(cm, expectedCM); d != "" {
		t.Errorf("failed to update additional pac controller route %s", diff.PrintWantGot(d))
	}
}

func TestDropConfigMapDataKeys(t *testing.T) {
	manifest := configMapManifest(t, pipelinesAsCodeCM, map[string]string{
		"hub-url":          "https://artifacthub.io",
		hubCatalogTypeKey:  "artifacthub",
		"application-name": "Pipelines as Code CI",
	})

	updated, err := manifest.Transform(dropConfigMapDataKeys(pipelinesAsCodeCM, hubCatalogTypeKey))
	assert.NilError(t, err)

	cm := configMapFromManifest(t, updated)
	_, exists := cm.Data[hubCatalogTypeKey]
	assert.DeepEqual(t, exists, false)
	assert.DeepEqual(t, cm.Data["hub-url"], "https://artifacthub.io")
	assert.DeepEqual(t, cm.Data["application-name"], "Pipelines as Code CI")

	other := configMapManifest(t, "other-config", map[string]string{hubCatalogTypeKey: "artifacthub"})
	unchanged, err := other.Transform(dropConfigMapDataKeys(pipelinesAsCodeCM, hubCatalogTypeKey))
	assert.NilError(t, err)
	assert.DeepEqual(t, configMapFromManifest(t, unchanged).Data[hubCatalogTypeKey], "artifacthub")
}

func TestUpdateAdditionControllerConfigMapDropsHubCatalogType(t *testing.T) {
	manifest := configMapManifest(t, pipelinesAsCodeCM, map[string]string{
		hubCatalogTypeKey: "artifacthub",
		"hub-url":         "https://artifacthub.io/api/v1",
	})

	updated, err := manifest.Transform(updateAdditionControllerConfigMap(v1alpha1.AdditionalPACControllerConfig{
		ConfigMapName: "custom-pac",
		Settings:      map[string]string{"hub-url": "https://artifacthub.io"},
	}))
	assert.NilError(t, err)

	cm := configMapFromManifest(t, updated)
	assert.DeepEqual(t, cm.GetName(), "custom-pac")
	_, exists := cm.Data[hubCatalogTypeKey]
	assert.DeepEqual(t, exists, false)
	assert.DeepEqual(t, cm.Data["hub-url"], "https://artifacthub.io")
}

func TestRemoveHubCatalogTypeFromConfigMap(t *testing.T) {
	ctx := context.Background()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: pipelinesAsCodeCM, Namespace: "openshift-pipelines"},
		Data: map[string]string{
			"hub-url":          "https://artifacthub.io",
			hubCatalogTypeKey:  "artifacthub",
			"application-name": "Pipelines as Code CI",
		},
	}
	client := k8sfake.NewSimpleClientset(cm)

	assert.NilError(t, RemoveHubCatalogTypeFromConfigMap(ctx, client, "openshift-pipelines"))

	got, err := client.CoreV1().ConfigMaps("openshift-pipelines").Get(ctx, pipelinesAsCodeCM, metav1.GetOptions{})
	assert.NilError(t, err)
	_, exists := got.Data[hubCatalogTypeKey]
	assert.DeepEqual(t, exists, false)
	assert.DeepEqual(t, got.Data["hub-url"], "https://artifacthub.io")
	assert.DeepEqual(t, got.Data["application-name"], "Pipelines as Code CI")

	assert.NilError(t, RemoveHubCatalogTypeFromConfigMap(ctx, client, "openshift-pipelines"))
	assert.NilError(t, RemoveHubCatalogTypeFromConfigMap(ctx, k8sfake.NewSimpleClientset(), "openshift-pipelines"))
	assert.NilError(t, RemoveHubCatalogTypeFromConfigMap(ctx, client, ""))
}

func configMapManifest(t *testing.T, name string, data map[string]string) mf.Manifest {
	t.Helper()
	cm := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	assert.NilError(t, err)
	u := unstructured.Unstructured{Object: obj}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{u}))
	assert.NilError(t, err)
	return manifest
}

func configMapFromManifest(t *testing.T, manifest mf.Manifest) *corev1.ConfigMap {
	t.Helper()
	cm := &corev1.ConfigMap{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assert.NilError(t, err)
	return cm
}
