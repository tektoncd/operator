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

package tektonconfig

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	k8sTesting "k8s.io/client-go/testing"
	"knative.dev/pkg/logging"
)

// reactor required for the GenerateName field to work when using the fake client
func generateNameReactor(action k8sTesting.Action) (bool, apimachineryRuntime.Object, error) {
	resource := action.(k8sTesting.CreateAction).GetObject()
	meta, ok := resource.(metav1.Object)
	if !ok {
		return false, resource, nil
	}

	if meta.GetName() == "" && meta.GetGenerateName() != "" {
		meta.SetName(common.SimpleNameGenerator.RestrictLengthWithRandomSuffix(meta.GetGenerateName()))
	}
	return false, resource, nil
}

func TestPostReconcileManifest(t *testing.T) {
	defaultConsolePluginImage := "ghcr.io/openshift-pipelines/console-plugin:main"

	tests := []struct {
		name               string
		consolePluginImage string
		operatorVersion    string
		targetNamespace    string
		tcConfig           *v1alpha1.Config
	}{
		{
			name:            "test-without-console-plugin-image",
			operatorVersion: "1.14.0",
			targetNamespace: "foo",
		},
		{
			name:               "test-with-console-plugin-image",
			consolePluginImage: "custom-image:tag1",
			operatorVersion:    "0.70.0",
			targetNamespace:    "bar",
		},
		{
			name:               "test-with-tc-config",
			consolePluginImage: "custom-image:tag1",
			operatorVersion:    "0.70.0",
			targetNamespace:    "bar",
			tcConfig: &v1alpha1.Config{
				NodeSelector: map[string]string{
					"node-role.kubernetes.io/infra": "",
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      "node-role.kubernetes.io/infra",
						Effect:   corev1.TaintEffectNoSchedule,
						Operator: corev1.TolerationOpExists,
					},
				},
				PriorityClassName: "high-priority",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			ctx := context.TODO()
			operatorFakeClientSet := fake.NewSimpleClientset()

			// add reactor to update generateName
			operatorFakeClientSet.PrependReactor("create", "*", generateNameReactor)

			// TEST: verifies required values in generated manifests (InstallerSet)
			verifyManifestFunc := func(expectedImage, expectedOperatorVersion string) {
				// verify installersets availability
				installerSetList, err := operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().List(
					ctx,
					metav1.ListOptions{LabelSelector: fmt.Sprintf("operator.tekton.dev/created-by=%s", consolePluginReconcileLabelCreatedByValue)},
				)
				require.NoError(t, err)

				require.Equal(t, 1, len(installerSetList.Items))
				installerSet := installerSetList.Items[0]

				// verify operator version label
				operatorVersion := installerSet.GetLabels()[v1alpha1.ReleaseVersionKey]
				require.Equal(t, expectedOperatorVersion, operatorVersion)

				// get installerset and verify transform values
				for _, u := range installerSet.Spec.Manifests {
					// verify targetNamespace
					require.Equal(t, test.targetNamespace, u.GetNamespace())

					switch u.GetKind() {
					case "Deployment":
						deployment := &appsv1.Deployment{}
						err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deployment)
						require.NoError(t, err)
						require.Equal(t, "pipelines-console-plugin", deployment.GetName())
						container := deployment.Spec.Template.Spec.Containers[0]
						require.Equal(t, expectedImage, container.Image)
						// verify the config present on the deployment (on pod template)
						if test.tcConfig != nil {
							require.Equal(t, test.tcConfig.NodeSelector, deployment.Spec.Template.Spec.NodeSelector, "nodeSelector mismatch on pod template")
							require.Equal(t, test.tcConfig.Tolerations, deployment.Spec.Template.Spec.Tolerations, "tolerations mismatch on pod template")
							require.Equal(t, test.tcConfig.PriorityClassName, deployment.Spec.Template.Spec.PriorityClassName, "priorityClass mismatch on pod template")
						}

					case "ConsolePlugin":
						actualNamespace, found, err := unstructured.NestedString(u.Object, "spec", "backend", "service", "namespace")
						require.NoError(t, err)
						require.True(t, found)
						require.Equal(t, test.targetNamespace, actualNamespace)

					}
				}
			}

			// reconciler reference
			postReconcile := &consolePluginReconciler{
				logger:                 logging.FromContext(ctx).Named("post-reconcile-manifest-test"),
				operatorClientSet:      operatorFakeClientSet,
				syncOnce:               sync.Once{},
				resourcesYamlDirectory: "./testdata/postreconcile_manifest",
				operatorVersion:        test.operatorVersion,
			}

			// tekton config CR
			tektonConfigCR := &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: test.targetNamespace,
					},
				},
			}

			// include tektonConfig.config
			if test.tcConfig != nil {
				tektonConfigCR.Spec.Config = *test.tcConfig.DeepCopy()
			}

			// console plugin image
			consolePluginImage := defaultConsolePluginImage
			// update image env variable
			if test.consolePluginImage != "" {
				t.Setenv("IMAGE_PIPELINES_CONSOLE_PLUGIN", test.consolePluginImage)
				consolePluginImage = test.consolePluginImage
			}
			// TEST: image name
			err := postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, test.operatorVersion) // verify manifests

			// TEST: operator version change
			// update operator version in installerSet and reconcile
			postReconcile.operatorVersion = "foo"
			err = postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, "foo") // verify
			postReconcile.operatorVersion = test.operatorVersion

			// TEST: removal of extra installerSet
			// add another tekton config manifest post reconcile installerset and reconcile
			newInstallerSet := &v1alpha1.TektonInstallerSet{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "another-tekton-config-manifest-foo-",
					Labels:       consolePluginReconcileInstallerSetLabel.MatchLabels,
				},
			}
			_, err = operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, newInstallerSet, metav1.CreateOptions{})
			require.NoError(t, err)
			err = postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, test.operatorVersion) // verify manifests

			// TEST: do not touch others installerSets
			// add another installerset(not tekton config manifest post reconcile) and reconcile
			// this installerSet should not be removed
			anotherInstallerSetName := "pipelines-foo"
			anotherInstallerSet := &v1alpha1.TektonInstallerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: anotherInstallerSetName,
				},
			}
			_, err = operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, anotherInstallerSet, metav1.CreateOptions{})
			require.NoError(t, err)
			err = postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, test.operatorVersion) // verify manifests
			installerSetList, err := operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{})
			require.NoError(t, err)
			require.Equal(t, 2, len(installerSetList.Items))
			expectedInstallerSetFound := false
			for _, installerSet := range installerSetList.Items {
				if installerSet.GetName() == anotherInstallerSetName {
					expectedInstallerSetFound = true
					break
				}
			}
			require.True(t, expectedInstallerSetFound)
		})
	}
}

func TestConvertTLSVersionToNginx(t *testing.T) {
	ctx := context.TODO()
	reconciler := &consolePluginReconciler{
		logger: logging.FromContext(ctx).Named("test-convert-tls-version"),
	}

	tests := []struct {
		name           string
		tlsVersion     string
		expectedOutput string
	}{
		{
			name:           "VersionTLS13",
			tlsVersion:     "VersionTLS13",
			expectedOutput: "TLSv1.3",
		},
		{
			name:           "VersionTLS12",
			tlsVersion:     "VersionTLS12",
			expectedOutput: "TLSv1.2 TLSv1.3",
		},
		{
			name:           "VersionTLS11",
			tlsVersion:     "VersionTLS11",
			expectedOutput: "TLSv1.1 TLSv1.2 TLSv1.3",
		},
		{
			name:           "VersionTLS10",
			tlsVersion:     "VersionTLS10",
			expectedOutput: "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3",
		},
		{
			name:           "unknown version defaults to safe",
			tlsVersion:     "UnknownVersion",
			expectedOutput: "TLSv1.2 TLSv1.3",
		},
		{
			name:           "empty version defaults to safe",
			tlsVersion:     "",
			expectedOutput: "TLSv1.2 TLSv1.3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := reconciler.convertTLSVersionToNginx(test.tlsVersion)
			require.Equal(t, test.expectedOutput, result)
		})
	}
}

func TestBuildNginxTLSDirectives(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name                string
		tlsMinVersion       string
		tlsCipherSuites     string
		tlsCurvePreferences string
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name:          "all TLS settings provided (cipher suites skipped)",
			tlsMinVersion: "VersionTLS13",
			tlsCipherSuites: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384",
			tlsCurvePreferences: "X25519,prime256v1",
			expectedContains: []string{
				"ssl_protocols TLSv1.3;",
				"ssl_ecdh_curve X25519:prime256v1;",
			},
			expectedNotContains: []string{
				"ssl_ciphers",
				"ssl_prefer_server_ciphers",
			},
		},
		{
			name:          "only min version provided",
			tlsMinVersion: "VersionTLS12",
			expectedContains: []string{
				"ssl_protocols TLSv1.2 TLSv1.3;",
			},
			expectedNotContains: []string{
				"ssl_ciphers",
				"ssl_ecdh_curve",
			},
		},
		{
			name:            "only cipher suites provided (skipped, no output)",
			tlsMinVersion:   "",
			tlsCipherSuites: "TLS_AES_128_GCM_SHA256",
			expectedContains: []string{},
			expectedNotContains: []string{
				"ssl_ciphers",
				"ssl_prefer_server_ciphers",
			},
		},
		{
			name:                "only curve preferences provided",
			tlsMinVersion:       "",
			tlsCurvePreferences: "X25519",
			expectedContains: []string{
				"ssl_ecdh_curve X25519;",
			},
		},
		{
			name:                "no TLS settings",
			tlsMinVersion:       "",
			tlsCipherSuites:     "",
			tlsCurvePreferences: "",
			expectedContains:    []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := &consolePluginReconciler{
				logger:              logging.FromContext(ctx).Named("test-build-directives"),
				tlsMinVersion:       test.tlsMinVersion,
				tlsCipherSuites:     test.tlsCipherSuites,
				tlsCurvePreferences: test.tlsCurvePreferences,
			}

			result := reconciler.buildNginxTLSDirectives()

			// Check expected content
			for _, expected := range test.expectedContains {
				require.Contains(t, result, expected, "Expected directive not found")
			}

			// Check unexpected content
			for _, notExpected := range test.expectedNotContains {
				require.NotContains(t, result, notExpected, "Unexpected directive found")
			}
		})
	}
}

func TestGenerateNginxConfWithTLS(t *testing.T) {
	ctx := context.TODO()

	baseNginxConf := `error_log /dev/stdout warn;
events {}
http {
  access_log         /dev/stdout;
  include            /etc/nginx/mime.types;
  default_type       application/octet-stream;
  keepalive_timeout  65;
  server {
    listen              8443 ssl;
    listen              [::]:8443 ssl;
    ssl_certificate     /var/cert/tls.crt;
    ssl_certificate_key /var/cert/tls.key;
    root                /usr/share/nginx/html;
  }
}`

	tests := []struct {
		name                string
		tlsMinVersion       string
		tlsCipherSuites     string
		tlsCurvePreferences string
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name:          "with TLS configuration (cipher suites skipped)",
			tlsMinVersion: "VersionTLS12",
			tlsCipherSuites: "TLS_AES_128_GCM_SHA256",
			tlsCurvePreferences: "X25519",
			expectedContains: []string{
				"server {",
				"ssl_protocols TLSv1.2 TLSv1.3;",
				"ssl_ecdh_curve X25519;",
				"listen              8443 ssl;",
				"ssl_certificate     /var/cert/tls.crt;",
			},
			expectedNotContains: []string{
				"ssl_ciphers",
				"ssl_prefer_server_ciphers",
			},
		},
		{
			name:          "without TLS configuration returns original",
			tlsMinVersion: "",
			expectedContains: []string{
				"server {",
				"listen              8443 ssl;",
				"ssl_certificate     /var/cert/tls.crt;",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := &consolePluginReconciler{
				logger:              logging.FromContext(ctx).Named("test-generate-conf"),
				tlsMinVersion:       test.tlsMinVersion,
				tlsCipherSuites:     test.tlsCipherSuites,
				tlsCurvePreferences: test.tlsCurvePreferences,
			}

			result := reconciler.generateNginxConfWithTLS(baseNginxConf)

			// Verify TLS directives are injected after "server {"
			for _, expected := range test.expectedContains {
				require.Contains(t, result, expected, "Expected content not found in generated nginx.conf")
			}

			// Check unexpected content
			for _, notExpected := range test.expectedNotContains {
				require.NotContains(t, result, notExpected, "Unexpected directive found in generated nginx.conf")
			}

			// Verify TLS directives come after "server {" line
			if test.tlsMinVersion != "" {
				serverBlockStart := "server {"
				sslProtocolsLine := "ssl_protocols"
				serverIndex := len(result[:len(result)])
				protocolsIndex := len(result[:len(result)])
				
				for i := 0; i < len(result)-len(serverBlockStart); i++ {
					if result[i:i+len(serverBlockStart)] == serverBlockStart && serverIndex == len(result) {
						serverIndex = i
					}
				}
				
				for i := 0; i < len(result)-len(sslProtocolsLine); i++ {
					if result[i:i+len(sslProtocolsLine)] == sslProtocolsLine && protocolsIndex == len(result) {
						protocolsIndex = i
					}
				}
				
				if serverIndex < len(result) && protocolsIndex < len(result) {
					require.Greater(t, protocolsIndex, serverIndex, "ssl_protocols should appear after 'server {' block")
				}
			}
		})
	}
}

func TestTransformerNginxTLS(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name                string
		tlsMinVersion       string
		tlsCipherSuites     string
		tlsCurvePreferences string
		inputConfigMap      *unstructured.Unstructured
		expectedError       bool
		expectedContains    []string
	}{
		{
			name:          "transform nginx ConfigMap with TLS (cipher suites skipped)",
			tlsMinVersion: "VersionTLS13",
			tlsCipherSuites: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384",
			inputConfigMap: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "pipelines-console-plugin",
						"namespace": "openshift-pipelines",
					},
					"data": map[string]interface{}{
						"nginx.conf": `server {
  listen 8443 ssl;
}`,
					},
				},
			},
			expectedContains: []string{
				"ssl_protocols TLSv1.3;",
			},
		},
		{
			name:          "skip non-ConfigMap resources",
			tlsMinVersion: "VersionTLS12",
			inputConfigMap: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "test-deployment",
					},
				},
			},
			expectedError: false,
		},
		{
			name:          "skip other ConfigMaps",
			tlsMinVersion: "VersionTLS12",
			inputConfigMap: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "other-configmap",
					},
					"data": map[string]interface{}{
						"some-key": "some-value",
					},
				},
			},
			expectedError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := &consolePluginReconciler{
				logger:              logging.FromContext(ctx).Named("test-transformer"),
				tlsMinVersion:       test.tlsMinVersion,
				tlsCipherSuites:     test.tlsCipherSuites,
				tlsCurvePreferences: test.tlsCurvePreferences,
			}

			transformer := reconciler.transformerNginxTLS()
			err := transformer(test.inputConfigMap)

			if test.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Verify transformed nginx.conf if it's the pipelines-console-plugin ConfigMap
			if test.inputConfigMap.GetKind() == "ConfigMap" && test.inputConfigMap.GetName() == "pipelines-console-plugin" {
				nginxConf, found, err := unstructured.NestedString(test.inputConfigMap.Object, "data", "nginx.conf")
				require.NoError(t, err)
				require.True(t, found)

				for _, expected := range test.expectedContains {
					require.Contains(t, nginxConf, expected, "Expected TLS directive not found in transformed nginx.conf")
				}
			}
		})
	}
}

func TestNginxTLSIntegration(t *testing.T) {
	ctx := context.TODO()
	operatorFakeClientSet := fake.NewSimpleClientset()
	operatorFakeClientSet.PrependReactor("create", "*", generateNameReactor)

	tests := []struct {
		name                string
		tlsMinVersion       string
		tlsCipherSuites     string
		tlsCurvePreferences string
		expectedTLSInNginx  []string
	}{
		{
			name:          "integration test with full TLS config (cipher suites skipped)",
			tlsMinVersion: "VersionTLS12",
			tlsCipherSuites: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384",
			tlsCurvePreferences: "X25519,prime256v1",
			expectedTLSInNginx: []string{
				"ssl_protocols TLSv1.2 TLSv1.3;",
				"ssl_ecdh_curve X25519:prime256v1;",
			},
		},
		{
			name:               "integration test with fail-safe defaults",
			tlsMinVersion:      "", // Empty to trigger fail-safe
			expectedTLSInNginx: []string{
				"ssl_protocols TLSv1.2 TLSv1.3;", // Safe default should be applied
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set environment variables for TLS configuration
			if test.tlsMinVersion != "" {
				t.Setenv(TLSMinVersionEnvKey, test.tlsMinVersion)
			}
			if test.tlsCipherSuites != "" {
				t.Setenv(TLSCipherSuitesEnvKey, test.tlsCipherSuites)
			}
			if test.tlsCurvePreferences != "" {
				t.Setenv(TLSCurvePreferencesEnvKey, test.tlsCurvePreferences)
			}

			reconciler := &consolePluginReconciler{
				logger:                 logging.FromContext(ctx).Named("integration-test"),
				operatorClientSet:      operatorFakeClientSet,
				syncOnce:               sync.Once{},
				resourcesYamlDirectory: "./testdata/postreconcile_manifest",
				operatorVersion:        "test-version",
			}

			tektonConfigCR := &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: "openshift-pipelines",
					},
				},
			}

			err := reconciler.reconcile(ctx, tektonConfigCR)
			require.NoError(t, err)

			// Verify the InstallerSet was created
			installerSetList, err := operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().List(
				ctx,
				metav1.ListOptions{LabelSelector: fmt.Sprintf("operator.tekton.dev/created-by=%s", consolePluginReconcileLabelCreatedByValue)},
			)
			require.NoError(t, err)
			require.Equal(t, 1, len(installerSetList.Items))

			// Find the nginx ConfigMap in the manifests
			installerSet := installerSetList.Items[0]
			var nginxConfigMap *unstructured.Unstructured
			for _, manifest := range installerSet.Spec.Manifests {
				if manifest.GetKind() == "ConfigMap" && manifest.GetName() == "pipelines-console-plugin" {
					nginxConfigMap = &manifest
					break
				}
			}

			require.NotNil(t, nginxConfigMap, "nginx ConfigMap not found in InstallerSet manifests")

			// Extract nginx.conf and verify TLS directives
			nginxConf, found, err := unstructured.NestedString(nginxConfigMap.Object, "data", "nginx.conf")
			require.NoError(t, err)
			require.True(t, found, "nginx.conf not found in ConfigMap")

			// Verify expected TLS directives
			for _, expected := range test.expectedTLSInNginx {
				require.Contains(t, nginxConf, expected, "Expected TLS directive not found in nginx.conf")
			}
		})
	}
}
