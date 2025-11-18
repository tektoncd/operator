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
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	fake2 "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func Test_isEnableRoute(t *testing.T) {
	tests := []struct {
		name         string
		routeEnabled *bool
		want         bool
	}{
		{name: "route enabled", routeEnabled: ptr.Bool(true), want: true},
		{name: "route disabled", routeEnabled: ptr.Bool(false), want: false},
		{name: "route nil defaults to false", routeEnabled: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &v1alpha1.TektonResult{
				Spec: v1alpha1.TektonResultSpec{
					Result: v1alpha1.Result{
						ResultsAPIProperties: v1alpha1.ResultsAPIProperties{
							RouteEnabled: tt.routeEnabled,
						},
					},
				},
			}
			assert.Equal(t, isEnableRoute(result), tt.want)
		})
	}
}

func Test_PostReconcile_RouteToggle(t *testing.T) {
	os.Setenv(common.KoEnvKey, "testdata")

	tests := []struct {
		name                 string
		routeEnabled         *bool
		expectError          error
		expectInstallerCount int
	}{
		{
			name:                 "route disabled - cleanup postset",
			routeEnabled:         ptr.Bool(false),
			expectError:          nil,
			expectInstallerCount: 0,
		},
		{
			name:                 "route enabled - create postset",
			routeEnabled:         ptr.Bool(true),
			expectError:          v1alpha1.REQUEUE_EVENT_AFTER,
			expectInstallerCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			routeManifest, err := getRouteManifest()
			assertNoError(t, err)

			result := &v1alpha1.TektonResult{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-result",
					Namespace: "test-ns",
				},
				Spec: v1alpha1.TektonResultSpec{
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Result: v1alpha1.Result{
						ResultsAPIProperties: v1alpha1.ResultsAPIProperties{
							RouteEnabled: tt.routeEnabled,
						},
					},
				},
			}

			fakeClient := fake2.NewSimpleClientset()
			installerSetClient := client.NewInstallerSetClient(
				fakeClient.OperatorV1alpha1().TektonInstallerSets(),
				"v0.0.1",
				"results-ext",
				v1alpha1.KindTektonResult,
				nil,
			)

			ext := &openshiftExtension{
				installerSetClient: installerSetClient,
				routeManifest:      routeManifest,
				logsRBACManifest:   &mf.Manifest{},
			}

			err = ext.PostReconcile(ctx, result)
			assert.Equal(t, err, tt.expectError)

			list, err := fakeClient.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{})
			assertNoError(t, err)
			assert.Equal(t, len(list.Items), tt.expectInstallerCount)
		})
	}
}

func Test_injectPostgresUpgradeSupport(t *testing.T) {
	// Create a minimal postgres StatefulSet for testing
	postgresStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-results-postgres",
			Namespace: "tekton-pipelines",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "registry.redhat.io/rhel9/postgresql-15@sha256:test",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.Bool(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "postgredb",
									MountPath: "/var/lib/pgsql/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "postgredb",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "postgredb",
								},
							},
						},
					},
				},
			},
		},
	}

	// Convert to unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(postgresStatefulSet)
	assertNoError(t, err)

	// Create unstructured object
	u := &unstructured.Unstructured{Object: unstructuredMap}
	u.SetAPIVersion("apps/v1")
	u.SetKind("StatefulSet")

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{*u}))
	assertNoError(t, err)

	// Apply the transformer
	manifest, err = manifest.Transform(injectPostgresUpgradeSupport())
	assertNoError(t, err)

	// Convert back to StatefulSet
	statefulset := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, statefulset)
	assertNoError(t, err)

	// Verify main container modifications
	mainContainer := statefulset.Spec.Template.Spec.Containers[0]
	assert.Equal(t, mainContainer.Name, "postgres", "Main container name should not change")

	// Verify main container command was modified
	assert.Equal(t, len(mainContainer.Command), 2)
	assert.Equal(t, mainContainer.Command[0], "/bin/bash")
	assert.Equal(t, mainContainer.Command[1], "/upgrade-scripts/postgres-wrapper.sh")

	// Verify main container has upgrade-scripts volume mount
	mainVolumeNames := make(map[string]bool)
	for _, vm := range mainContainer.VolumeMounts {
		mainVolumeNames[vm.Name] = true
	}
	assert.Equal(t, mainVolumeNames["upgrade-scripts"], true, "Main container should mount upgrade-scripts volume")

	// Verify upgrade-scripts volume was added
	volumeFound := false
	for _, vol := range statefulset.Spec.Template.Spec.Volumes {
		if vol.Name == "upgrade-scripts" {
			volumeFound = true
			assert.Equal(t, vol.VolumeSource.ConfigMap != nil, true, "upgrade-scripts should be a ConfigMap volume")
			assert.Equal(t, vol.VolumeSource.ConfigMap.Name, "postgres-upgrade-scripts")
			assert.Equal(t, *vol.VolumeSource.ConfigMap.DefaultMode, int32(0755))
		}
	}
	assert.Equal(t, volumeFound, true, "upgrade-scripts volume should be added")
}

func Test_injectPostgresUpgradeSupport_SkipsNonPostgresStatefulSet(t *testing.T) {
	// Create a non-postgres StatefulSet
	otherStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-statefulset",
			Namespace: "tekton-pipelines",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "some-image:latest",
						},
					},
				},
			},
		},
	}

	// Convert to unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(otherStatefulSet)
	assertNoError(t, err)

	// Create unstructured object
	u := &unstructured.Unstructured{Object: unstructuredMap}
	u.SetAPIVersion("apps/v1")
	u.SetKind("StatefulSet")

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{*u}))
	assertNoError(t, err)

	originalInitContainerCount := len(otherStatefulSet.Spec.Template.Spec.InitContainers)
	originalVolumeCount := len(otherStatefulSet.Spec.Template.Spec.Volumes)

	// Apply the transformer
	manifest, err = manifest.Transform(injectPostgresUpgradeSupport())
	assertNoError(t, err)

	// Convert back to StatefulSet
	statefulset := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, statefulset)
	assertNoError(t, err)

	// Verify nothing was modified (since name doesn't match "tekton-results-postgres")
	assert.Equal(t, len(statefulset.Spec.Template.Spec.InitContainers), originalInitContainerCount,
		"Init containers should not be modified for non-postgres statefulset")
	assert.Equal(t, len(statefulset.Spec.Template.Spec.Volumes), originalVolumeCount,
		"Volumes should not be modified for non-postgres statefulset")
}

func Test_injectPostgresUpgradeSupport_SkipsNonStatefulSet(t *testing.T) {
	// Create a Deployment (not a StatefulSet)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-results-api",
			Namespace: "tekton-pipelines",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: "results-api:latest",
						},
					},
				},
			},
		},
	}

	// Convert to unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
	assertNoError(t, err)

	// Create unstructured object
	u := &unstructured.Unstructured{Object: unstructuredMap}
	u.SetAPIVersion("apps/v1")
	u.SetKind("Deployment")

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{*u}))
	assertNoError(t, err)

	originalInitContainerCount := len(deployment.Spec.Template.Spec.InitContainers)
	originalVolumeCount := len(deployment.Spec.Template.Spec.Volumes)

	// Apply the transformer - should skip Deployment resources
	manifest, err = manifest.Transform(injectPostgresUpgradeSupport())
	assertNoError(t, err)

	// Convert back to Deployment
	deploymentResult := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deploymentResult)
	assertNoError(t, err)

	// Verify nothing was modified (not a StatefulSet)
	assert.Equal(t, len(deploymentResult.Spec.Template.Spec.InitContainers), originalInitContainerCount,
		"Init containers should not be modified for Deployment resources")
	assert.Equal(t, len(deploymentResult.Spec.Template.Spec.Volumes), originalVolumeCount,
		"Volumes should not be modified for Deployment resources")
}

func Test_injectPostgresUpgradeSupport_Idempotent(t *testing.T) {
	postgresStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-results-postgres",
			Namespace: "tekton-pipelines",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "registry.redhat.io/rhel9/postgresql-15@sha256:test",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.Bool(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "postgredb",
									MountPath: "/var/lib/pgsql/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "postgredb",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "postgredb",
								},
							},
						},
					},
				},
			},
		},
	}

	// Convert to unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(postgresStatefulSet)
	assertNoError(t, err)

	// Create unstructured object
	u := &unstructured.Unstructured{Object: unstructuredMap}
	u.SetAPIVersion("apps/v1")
	u.SetKind("StatefulSet")

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{*u}))
	assertNoError(t, err)

	// Apply the transformer twice
	manifest, err = manifest.Transform(injectPostgresUpgradeSupport())
	assertNoError(t, err)

	statefulset := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, statefulset)
	assertNoError(t, err)

	firstInitContainerCount := len(statefulset.Spec.Template.Spec.InitContainers)
	firstVolumeCount := len(statefulset.Spec.Template.Spec.Volumes)

	// Apply again
	manifest, err = manifest.Transform(injectPostgresUpgradeSupport())
	assertNoError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, statefulset)
	assertNoError(t, err)

	// Verify counts remain the same (idempotent operation)
	assert.Equal(t, len(statefulset.Spec.Template.Spec.InitContainers), firstInitContainerCount,
		"Transformer should be idempotent - init container count should not change on second application")
	assert.Equal(t, len(statefulset.Spec.Template.Spec.Volumes), firstVolumeCount,
		"Transformer should be idempotent - volume count should not change on second application")
}
