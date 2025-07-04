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
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestFilterAdditionalControllerManifest(t *testing.T) {
	// Table-driven test cases for OpenShift and Kubernetes
	tests := []struct {
		name                      string
		isOpenShift               bool
		expectedResourceLen       int
		expectedRouteLen          int
		expectedServiceMonitorLen int
	}{
		{
			name:                      "OpenShift Platform",
			isOpenShift:               true,
			expectedResourceLen:       5, // Expect 5 resources in OpenShift (Deployment, Service, Route, ConfigMap, ServiceMonitor)
			expectedRouteLen:          1,
			expectedServiceMonitorLen: 1,
		},
		{
			name:                      "Kubernetes Platform",
			isOpenShift:               false,
			expectedResourceLen:       4, // Expect 4 resources in Kubernetes (Deployment, Service, ConfigMap, ServiceMonitor)
			expectedRouteLen:          0,
			expectedServiceMonitorLen: 1,
		},
	}

	// Loop through each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the isOpenShiftPlatform function for the test case
			orig := isOpenShiftPlatform
			isOpenShiftPlatform = func() bool { return tt.isOpenShift }
			defer func() { isOpenShiftPlatform = orig }()

			// Load the test data and filter the manifest
			manifest, err := mf.ManifestFrom(mf.Recursive(path.Join("testdata", "test-filter-manifest.yaml")))
			assert.NilError(t, err)

			// Apply the filter
			filtered := filterAdditionalControllerManifest(manifest)

			// Assert the expected number of resources
			assert.DeepEqual(t, len(filtered.Resources()), tt.expectedResourceLen)

			// Assert that the Route is present/absent depending on the platform
			routes := filtered.Filter(mf.All(mf.ByKind("Route")))
			assert.DeepEqual(t, len(routes.Resources()), tt.expectedRouteLen)

			// Assert that the ServiceMonitor is present in both cases
			sms := filtered.Filter(mf.All(mf.ByKind("ServiceMonitor")))
			assert.DeepEqual(t, len(sms.Resources()), tt.expectedServiceMonitorLen)
		})
	}
}

func TestUpdateAdditionControllerDeployment(t *testing.T) {
	tests := []struct {
		name                string
		isOpenShift         bool
		expectedResourceLen int
	}{
		{
			name:                "OpenShift Platform",
			isOpenShift:         true,
			expectedResourceLen: 1, // Expect the specific deployment resource to be updated
		},
		{
			name:                "Kubernetes Platform",
			isOpenShift:         false,
			expectedResourceLen: 1, // Expect the same behavior in Kubernetes
		},
	}

	// Loop through each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the isOpenShiftPlatform function for the test case
			orig := isOpenShiftPlatform
			isOpenShiftPlatform = func() bool { return tt.isOpenShift }
			defer func() { isOpenShiftPlatform = orig }()

			// Load the test data and filter the manifest
			testData := path.Join("testdata", "test-filter-manifest.yaml")
			manifest, err := mf.ManifestFrom(mf.Recursive(testData))
			assert.NilError(t, err)

			// Apply the filter
			manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Deployment")))

			additionalPACConfig := v1alpha1.AdditionalPACControllerConfig{
				ConfigMapName: "test-configmap",
				SecretName:    "test-secret",
			}

			updatedDeployment, err := manifest.Transform(updateAdditionControllerDeployment(additionalPACConfig, "test"))
			assert.NilError(t, err)

			// Check that deployment is updated
			assert.DeepEqual(t, updatedDeployment.Resources()[0].GetName(), "test-pac-controller")
		})
	}
}

func TestUpdateAdditionControllerService(t *testing.T) {
	tests := []struct {
		name                string
		isOpenShift         bool
		expectedResourceLen int
	}{
		{
			name:                "OpenShift Platform",
			isOpenShift:         true,
			expectedResourceLen: 1, // Expect specific service resource to be updated
		},
		{
			name:                "Kubernetes Platform",
			isOpenShift:         false,
			expectedResourceLen: 1, // Expect same behavior in Kubernetes
		},
	}

	// Loop through each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the isOpenShiftPlatform function for the test case
			orig := isOpenShiftPlatform
			isOpenShiftPlatform = func() bool { return tt.isOpenShift }
			defer func() { isOpenShiftPlatform = orig }()

			// Load the test data and filter the manifest
			testData := path.Join("testdata", "test-filter-manifest.yaml")
			manifest, err := mf.ManifestFrom(mf.Recursive(testData))
			assert.NilError(t, err)
			manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Service")))

			// Apply the filter for Service
			updatedManifest, err := manifest.Transform(updateAdditionControllerService("test"))
			assert.NilError(t, err)

			// Assert that the Service is updated
			assert.DeepEqual(t, updatedManifest.Resources()[0].GetName(), "test-pac-controller")
		})
	}
}

func TestUpdateAdditionControllerRoute(t *testing.T) {
	tests := []struct {
		name                string
		isOpenShift         bool
		expectedResourceLen int
	}{
		{
			name:                "OpenShift Platform",
			isOpenShift:         true,
			expectedResourceLen: 1, // Expect route to be updated in OpenShift
		},
		{
			name:                "Kubernetes Platform",
			isOpenShift:         false,
			expectedResourceLen: 0, // No route in Kubernetes
		},
	}

	// Loop through each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the isOpenShiftPlatform function for the test case
			orig := isOpenShiftPlatform
			isOpenShiftPlatform = func() bool { return tt.isOpenShift }
			defer func() { isOpenShiftPlatform = orig }()

			// Load the test data and filter the manifest
			testData := path.Join("testdata", "test-filter-manifest.yaml")
			manifest, err := mf.ManifestFrom(mf.Recursive(testData))
			assert.NilError(t, err)
			manifest = manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Route")))

			// Apply the filter for Route
			updatedManifest, err := manifest.Transform(updateAdditionControllerRoute("test"))
			if err != nil {
				assert.NilError(t, err)
			}

			// Assert Route is updated (or not, depending on platform)
			route := &routev1.Route{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(updatedManifest.Resources()[0].Object, route)
			if err != nil {
				assert.NilError(t, err)
			}

			// Assert that route is updated correctly
			assert.DeepEqual(t, route.Spec.To.Name, "test-pac-controller")
		})
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
		Settings:      map[string]string{"application-name": "Test CI application", "hub-url": "https://custom-hub.com"},
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
