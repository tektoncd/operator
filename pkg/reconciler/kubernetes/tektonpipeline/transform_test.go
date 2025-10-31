/*
Copyright 2023 The Tekton Authors

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

package tektonpipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/pipeline/test/diff"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

func TestUpdateResolverConfigEnvironmentsInDeployment(t *testing.T) {
	pipelineCR := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline",
			Namespace: "xyz",
		},
	}

	depInput := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelinesRemoteResolversControllerDeployment,
			Namespace: "xyz",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "hello",
							Image: "xyz",
						},
						// the container index 1 is used in tests
						{
							Name:  pipelinesRemoteResolverControllerContainer,
							Image: "xyz",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name                  string
		getPipelineCR         func() *v1alpha1.TektonPipeline
		getDeployment         func() *appsv1.Deployment
		getExpectedDeployment func() *appsv1.Deployment
	}{
		// verify with empty config
		{
			name:          "test-with-empty",
			getPipelineCR: func() *v1alpha1.TektonPipeline { return pipelineCR.DeepCopy() },
			getDeployment: func() *appsv1.Deployment { return depInput.DeepCopy() },
			getExpectedDeployment: func() *appsv1.Deployment {
				return depInput.DeepCopy()
			},
		},

		// verifies with hub api url
		{
			name: "test-with-hup-api-url",
			getPipelineCR: func() *v1alpha1.TektonPipeline {
				cr := pipelineCR.DeepCopy()
				cr.Spec.ResolversConfig.HubResolverConfig = map[string]string{
					resolverEnvKeyTektonHubApi: "http://localhost:9090/api",
				}
				return cr
			},
			getDeployment: func() *appsv1.Deployment { return depInput.DeepCopy() },
			getExpectedDeployment: func() *appsv1.Deployment {
				dep := depInput.DeepCopy()
				dep.Spec.Template.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "TEKTON_HUB_API",
						Value: "http://localhost:9090/api",
					},
				}
				return dep
			},
		},

		// verifies with hub api and artifact api urls
		{
			name: "test-with-hup-all-url",
			getPipelineCR: func() *v1alpha1.TektonPipeline {
				cr := pipelineCR.DeepCopy()
				cr.Spec.ResolversConfig.HubResolverConfig = map[string]string{
					resolverEnvKeyTektonHubApi:   "http://localhost:9090/api",
					resolverEnvKeyArtifactHubApi: "https://artifact.example.com:8443",
				}
				return cr
			},
			getDeployment: func() *appsv1.Deployment { return depInput.DeepCopy() },
			getExpectedDeployment: func() *appsv1.Deployment {
				dep := depInput.DeepCopy()
				dep.Spec.Template.Spec.Containers[1].Env = []corev1.EnvVar{
					// order(name) is important
					{
						Name:  "ARTIFACT_HUB_API",
						Value: "https://artifact.example.com:8443",
					},
					{
						Name:  "TEKTON_HUB_API",
						Value: "http://localhost:9090/api",
					},
				}
				return dep
			},
		},

		// verifies with existing environment
		{
			name: "test-with-existing-env-and-hup-url",
			getPipelineCR: func() *v1alpha1.TektonPipeline {
				cr := pipelineCR.DeepCopy()
				cr.Spec.ResolversConfig.HubResolverConfig = map[string]string{
					resolverEnvKeyTektonHubApi: "http://localhost:9090/api",
				}
				return cr
			},
			getDeployment: func() *appsv1.Deployment {
				dep := depInput.DeepCopy()
				dep.Spec.Template.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "CUSTOM_ENV",
						Value: "hello",
					},
					{
						Name:  "TEKTON_HUB_API",
						Value: "https://hub.tekton.dev",
					},
				}
				return dep
			},
			getExpectedDeployment: func() *appsv1.Deployment {
				dep := depInput.DeepCopy()
				dep.Spec.Template.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "CUSTOM_ENV",
						Value: "hello",
					},
					{
						Name:  "TEKTON_HUB_API",
						Value: "http://localhost:9090/api",
					},
				}
				return dep
			},
		},

		// verifies with existing hub api env in different form
		{
			name: "test-with-existing-env-and-hup-url-from-config",
			getPipelineCR: func() *v1alpha1.TektonPipeline {
				cr := pipelineCR.DeepCopy()
				cr.Spec.ResolversConfig.HubResolverConfig = map[string]string{
					resolverEnvKeyTektonHubApi: "http://localhost:9090/api",
				}
				return cr
			},
			getDeployment: func() *appsv1.Deployment {
				dep := depInput.DeepCopy()
				dep.Spec.Template.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "CUSTOM_ENV",
						Value: "hello",
					},
					{
						Name: "TEKTON_HUB_API",
						ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								Key: "tekton-hub-api",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "custom-config-map",
								},
							},
						},
					},
					{
						Name:  "ARTIFACT_HUB_API",
						Value: "https://artifacthub.io",
					},
				}
				return dep
			},
			getExpectedDeployment: func() *appsv1.Deployment {
				dep := depInput.DeepCopy()
				dep.Spec.Template.Spec.Containers[1].Env = []corev1.EnvVar{
					{
						Name:  "CUSTOM_ENV",
						Value: "hello",
					},
					{
						Name:  "TEKTON_HUB_API",
						Value: "http://localhost:9090/api",
					},
					{
						Name:  "ARTIFACT_HUB_API",
						Value: "https://artifacthub.io",
					},
				}
				return dep
			},
		},
	}

	// execute tests
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pipelineCR := test.getPipelineCR()
			depInput := test.getDeployment()

			// convert to unstructured object
			jsonBytes, err := json.Marshal(&depInput)
			assert.NilError(t, err)
			ud := &unstructured.Unstructured{}
			err = json.Unmarshal(jsonBytes, ud)
			assert.NilError(t, err)

			// apply transformer
			transformer := updateResolverConfigEnvironmentsInDeployment(pipelineCR)
			err = transformer(ud)
			assert.NilError(t, err)

			// get transformed deployment
			depOut := &appsv1.Deployment{}
			err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, depOut)
			assert.NilError(t, err)

			depExpected := test.getExpectedDeployment()
			if d := cmp.Diff(depOut, depExpected); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

// not in use, see: https://github.com/tektoncd/pipeline/pull/7789
// this field is removed from pipeline component
// keeping in types to maintain the API compatibility
// this test verifies that, "EnableTektonOciBundles" always not present in the feature flags config map
func TestEnableTektonOciBundlesFeatureFlag(t *testing.T) {
	tp := &v1alpha1.TektonPipeline{
		Spec: v1alpha1.TektonPipelineSpec{
			Pipeline: v1alpha1.Pipeline{
				PipelineProperties: v1alpha1.PipelineProperties{
					EnableTektonOciBundles: ptr.Bool(true),
				},
			},
		},
	}
	ctx := context.TODO()

	tests := []struct {
		name                   string
		enableTektonOciBundles *bool
		expectedValue          string
	}{
		{name: "with-true", enableTektonOciBundles: ptr.Bool(true), expectedValue: "false"},
		{name: "with-false", enableTektonOciBundles: ptr.Bool(false), expectedValue: "false"},
		{name: "with-nil", enableTektonOciBundles: nil, expectedValue: "false"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tp.Spec.Pipeline.EnableTektonOciBundles = test.enableTektonOciBundles

			// get manifests
			manifest, err := common.Fetch("./testdata/tektonpipeline-feature-flags-base.yaml")
			assert.NilError(t, err, "error on fetching testdata")

			transformers := filterAndTransform(common.NoExtension(ctx))
			_, err = transformers(ctx, &manifest, tp)
			assert.NilError(t, err)

			resources := manifest.Resources()
			assert.Assert(t, len(resources) > 0)

			featureFlagsMap := corev1.ConfigMap{}
			err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(resources[0].Object, &featureFlagsMap)
			assert.NilError(t, err)

			flagValue, found := featureFlagsMap.Data["enable-tekton-oci-bundles"]
			assert.Assert(t, found == true, "'enable-tekton-oci-bundles' not found")
			assert.Assert(t, flagValue == test.expectedValue, "'enable-tekton-oci-bundles' is not '%s'", test.expectedValue)
		})
	}
}

// TestTracingConfiguration verifies that tracing properties are correctly
// populated in the config-tracing ConfigMap
func TestTracingConfiguration(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name              string
		tracingProperties v1alpha1.TracingProperties
		expectedData      map[string]string
	}{
		{
			name: "tracing-disabled",
			tracingProperties: v1alpha1.TracingProperties{
				Enabled: ptr.Bool(false),
			},
			expectedData: map[string]string{
				"enabled": "false",
			},
		},
		{
			name: "tracing-enabled-with-endpoint",
			tracingProperties: v1alpha1.TracingProperties{
				Enabled:  ptr.Bool(true),
				Endpoint: "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces",
			},
			expectedData: map[string]string{
				"enabled":  "true",
				"endpoint": "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces",
			},
		},
		{
			name: "tracing-enabled-with-otel-endpoint",
			tracingProperties: v1alpha1.TracingProperties{
				Enabled:  ptr.Bool(true),
				Endpoint: "http://otel-collector.observability.svc.cluster.local:4318/v1/traces",
			},
			expectedData: map[string]string{
				"enabled":  "true",
				"endpoint": "http://otel-collector.observability.svc.cluster.local:4318/v1/traces",
			},
		},
		{
			name: "tracing-with-credentials-secret",
			tracingProperties: v1alpha1.TracingProperties{
				Enabled:           ptr.Bool(true),
				Endpoint:          "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces",
				CredentialsSecret: "jaeger-auth-secret",
			},
			expectedData: map[string]string{
				"enabled":           "true",
				"endpoint":          "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces",
				"credentialsSecret": "jaeger-auth-secret",
			},
		},
		{
			name: "tracing-endpoint-only",
			tracingProperties: v1alpha1.TracingProperties{
				Endpoint: "http://custom-endpoint:8080/traces",
			},
			expectedData: map[string]string{
				"endpoint": "http://custom-endpoint:8080/traces",
			},
		},
		{
			name:              "tracing-empty",
			tracingProperties: v1alpha1.TracingProperties{},
			expectedData:      map[string]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tp := &v1alpha1.TektonPipeline{
				Spec: v1alpha1.TektonPipelineSpec{
					Pipeline: v1alpha1.Pipeline{
						PipelineProperties: v1alpha1.PipelineProperties{
							TracingProperties: test.tracingProperties,
						},
					},
				},
			}

			// get manifests
			manifest, err := common.Fetch("./testdata/tektonpipeline-config-tracing-base.yaml")
			assert.NilError(t, err, "error on fetching testdata")

			transformers := filterAndTransform(common.NoExtension(ctx))
			_, err = transformers(ctx, &manifest, tp)
			assert.NilError(t, err)

			resources := manifest.Resources()
			assert.Assert(t, len(resources) > 0)

			configTracingMap := corev1.ConfigMap{}
			err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(resources[0].Object, &configTracingMap)
			assert.NilError(t, err)

			// Verify expected data (excluding _example field)
			for key, expectedValue := range test.expectedData {
				actualValue, found := configTracingMap.Data[key]
				assert.Assert(t, found == true, "'%s' not found in config-tracing", key)
				assert.Assert(t, actualValue == expectedValue, "'%s' value is '%s', expected '%s'", key, actualValue, expectedValue)
			}

			// Verify no unexpected fields (excluding _example)
			for key := range configTracingMap.Data {
				if key == "_example" {
					continue
				}
				_, expected := test.expectedData[key]
				assert.Assert(t, expected == true, "unexpected field '%s' in config-tracing", key)
			}
		})
	}
}
