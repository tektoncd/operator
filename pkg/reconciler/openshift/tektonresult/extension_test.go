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
	"os"
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

func TestGetRouteManifest(t *testing.T) {
	os.Setenv(common.KoEnvKey, "notExist")
	_, err := getRouteManifest()
	if err == nil {
		t.Error("expected error, received no error")
	}

	os.Setenv(common.KoEnvKey, "testdata")
	mf, err := getRouteManifest()
	assertNoError(t, err)

	cr := &rbac.ClusterRole{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(mf.Resources()[0].Object, cr)
	assertNoError(t, err)

}

func assertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("assertion failed; expected no error %v", err)
	}
}

func TestGetLoggingRBACManifest(t *testing.T) {

	// Set expected manifest data in the testdata set with exact rbac manifest expected as mock data
	testData := path.Join("testdata", "static/tekton-results/logs-rbac/rbac.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	//Assert that the first resource of expected manifest is ClusterRole
	expectedCr := &rbac.ClusterRole{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expectedCr)
	assert.NilError(t, err)

	//Assert that the secound resource of expected manifest is ClusterRoleBinding
	expectedCrb := &rbac.ClusterRoleBinding{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[1].Object, expectedCrb)
	assert.NilError(t, err)

	// Invoke the function to get the actual mainfests
	returnedManifest, err := getloggingRBACManifest()
	//Assert that the function executes without error
	assert.NilError(t, err)

	//Assert that the first resource of returned manifest is ClusterRole
	returnedCr := &rbac.ClusterRole{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(returnedManifest.Resources()[0].Object, returnedCr)
	assert.NilError(t, err)

	//Assert that the first resource of returned manifest is ClusterRole
	returnedCrb := &rbac.ClusterRoleBinding{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(returnedManifest.Resources()[1].Object, returnedCrb)
	assert.NilError(t, err)

	//Assert that cluster role name matches between expected and returned
	assert.DeepEqual(t, expectedCr.GetName(), returnedCr.GetName())

	//Assert that cluster role binding name matches between expected and returned
	assert.DeepEqual(t, expectedCr.GetName(), returnedCr.GetName())

}

func Test_injecBoundSAToken(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)
	logsAPI := true
	props := v1alpha1.ResultsAPIProperties{
		LogsAPI:        &logsAPI,
		LogsType:       "File",
		LogsPath:       "logs",
		LoggingPVCName: "tekton-logs",
	}

	manifest, err = manifest.Transform(injectBoundSAToken(props))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	assert.Equal(t, deployment.Spec.Template.Spec.Volumes[2].Name, "bound-sa-token")
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name, "bound-sa-token")
}

func Test_injectLokiStackTLSCACert(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)
	props := v1alpha1.LokiStackProperties{
		LokiStackName:      "test",
		LokiStackNamespace: "bar",
	}

	manifest, err = manifest.Transform(injectLokiStackTLSCACert(props))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Env[5].Name, "LOGGING_PLUGIN_CA_CERT")

	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Env[5].ValueFrom.ConfigMapKeyRef.LocalObjectReference, corev1.LocalObjectReference{
		Name: "openshift-service-ca.crt",
	})

	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Env[5].ValueFrom.ConfigMapKeyRef.Key, "service-ca.crt")
}

func Test_injectResultsAPIServiceCACert(t *testing.T) {
	testData := path.Join("testdata", "api-service.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	service := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, service)
	assert.NilError(t, err)

	props := v1alpha1.ResultsAPIProperties{}
	manifest, err = manifest.Transform(injectResultsAPIServiceCACert(props))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, service)
	assert.NilError(t, err)

	assert.Equal(t, service.Annotations["service.beta.openshift.io/serving-cert-secret-name"], "tekton-results-tls")
}

func Test_ResultsAPIInjectRout(t *testing.T) {
	testData := path.Join("testdata", "static/tekton-results", "route-rbac", "rbac.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	filteredManifest := manifest.Filter(mf.ByKind("Route"))

	route := &routev1.Route{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(filteredManifest.Resources()[0].Object, route)
	assertNoError(t, err)

	props := v1alpha1.ResultsAPIProperties{RouteEnabled: ptr.Bool(true), RouteTLSTermination: "passthrough", RouteHost: "example.com", RoutePath: "/api"}
	manifest, err = filteredManifest.Transform(injectResultsAPIRoute(props))
	assert.NilError(t, err)

	route = &routev1.Route{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, route)
	assert.NilError(t, err)

	assert.Equal(t, route.Spec.TLS.Termination, routev1.TLSTerminationType("passthrough"))
	assert.Equal(t, route.Spec.Host, "example.com")
	assert.Equal(t, route.Spec.Path, "/api")
}
