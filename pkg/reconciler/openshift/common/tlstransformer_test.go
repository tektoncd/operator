/*
Copyright 2025 The Tekton Authors

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

package common

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestInjectTLSEnvVarsTransformer(t *testing.T) {
	// Setup test TLS config
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
		},
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
			tls.CurveP521,
		},
	}
	tlsHash := CalculateTLSConfigHash(tlsConfig)

	// Create context with TLS config
	ctx := context.Background()
	ctx = context.WithValue(ctx, tlsConfigKey{}, tlsConfig)
	ctx = context.WithValue(ctx, tlsConfigHashKey{}, tlsHash)

	tests := []struct {
		name           string
		ctx            context.Context //nolint:containedctx // Test struct needs context for transformer tests
		resource       *unstructured.Unstructured
		containerNames []string
		wantEnvVars    map[string][]corev1.EnvVar // containerName -> envVars
		wantAnnotation string
	}{
		{
			name: "Inject into Deployment - all containers",
			ctx:  ctx,
			resource: mustToUnstructured(&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-ns",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "api",
									Image: "test:latest",
								},
								{
									Name:  "sidecar",
									Image: "sidecar:latest",
								},
							},
						},
					},
				},
			}),
			containerNames: []string{}, // Empty means all containers
			wantEnvVars: map[string][]corev1.EnvVar{
				"api": {
					{Name: TLSMinVersionEnvVar, Value: "TLSv1.3"},
					{Name: TLSCipherSuitesEnvVar, Value: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
					{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384,P-521"},
				},
				"sidecar": {
					{Name: TLSMinVersionEnvVar, Value: "TLSv1.3"},
					{Name: TLSCipherSuitesEnvVar, Value: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
					{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384,P-521"},
				},
			},
			wantAnnotation: tlsHash,
		},
		{
			name: "Inject into Deployment - specific container",
			ctx:  ctx,
			resource: mustToUnstructured(&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-ns",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "api",
									Image: "test:latest",
								},
								{
									Name:  "sidecar",
									Image: "sidecar:latest",
								},
							},
						},
					},
				},
			}),
			containerNames: []string{"api"}, // Only inject into "api"
			wantEnvVars: map[string][]corev1.EnvVar{
				"api": {
					{Name: TLSMinVersionEnvVar, Value: "TLSv1.3"},
					{Name: TLSCipherSuitesEnvVar, Value: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
					{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384,P-521"},
				},
				"sidecar": nil, // Should not have TLS env vars
			},
			wantAnnotation: tlsHash,
		},
		{
			name: "Inject into StatefulSet",
			ctx:  ctx,
			resource: mustToUnstructured(&appsv1.StatefulSet{
				TypeMeta: metav1.TypeMeta{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "test-ns",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "postgres",
									Image: "postgres:15",
								},
							},
						},
					},
				},
			}),
			containerNames: []string{},
			wantEnvVars: map[string][]corev1.EnvVar{
				"postgres": {
					{Name: TLSMinVersionEnvVar, Value: "TLSv1.3"},
					{Name: TLSCipherSuitesEnvVar, Value: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
					{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384,P-521"},
				},
			},
			wantAnnotation: tlsHash,
		},
		{
			name: "Context without TLS config - no changes",
			ctx:  context.Background(), // No TLS config
			resource: mustToUnstructured(&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-ns",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "api",
									Image: "test:latest",
								},
							},
						},
					},
				},
			}),
			containerNames: []string{},
			wantEnvVars: map[string][]corev1.EnvVar{
				"api": nil, // No env vars should be added
			},
			wantAnnotation: "", // No annotation should be added
		},
		{
			name: "Existing env vars are preserved",
			ctx:  ctx,
			resource: mustToUnstructured(&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-ns",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "api",
									Image: "test:latest",
									Env: []corev1.EnvVar{
										{Name: "EXISTING_VAR", Value: "existing-value"},
									},
								},
							},
						},
					},
				},
			}),
			containerNames: []string{},
			wantEnvVars: map[string][]corev1.EnvVar{
				"api": {
					{Name: "EXISTING_VAR", Value: "existing-value"},
					{Name: TLSMinVersionEnvVar, Value: "TLSv1.3"},
					{Name: TLSCipherSuitesEnvVar, Value: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
					{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384,P-521"},
				},
			},
			wantAnnotation: tlsHash,
		},
		{
			name: "Existing TLS env vars are replaced",
			ctx:  ctx,
			resource: mustToUnstructured(&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test-ns",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "api",
									Image: "test:latest",
									Env: []corev1.EnvVar{
										{Name: TLSMinVersionEnvVar, Value: "TLSv1.2"}, // Old value
									},
								},
							},
						},
					},
				},
			}),
			containerNames: []string{},
			wantEnvVars: map[string][]corev1.EnvVar{
				"api": {
					{Name: TLSMinVersionEnvVar, Value: "TLSv1.3"}, // Updated
					{Name: TLSCipherSuitesEnvVar, Value: "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"},
					{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384,P-521"},
				},
			},
			wantAnnotation: tlsHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create transformer
			transformer := InjectTLSEnvVarsTransformer(tt.ctx, tt.containerNames...)

			// Apply transformer
			err := transformer(tt.resource)
			if err != nil {
				t.Fatalf("InjectTLSEnvVarsTransformer() error = %v", err)
			}

			// Convert back to typed object
			kind := tt.resource.GetKind()
			var containers []corev1.Container
			var annotations map[string]string

			switch kind {
			case "Deployment":
				d := &appsv1.Deployment{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(tt.resource.Object, d)
				if err != nil {
					t.Fatalf("Failed to convert to Deployment: %v", err)
				}
				containers = d.Spec.Template.Spec.Containers
				annotations = d.Spec.Template.Annotations

			case "StatefulSet":
				s := &appsv1.StatefulSet{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(tt.resource.Object, s)
				if err != nil {
					t.Fatalf("Failed to convert to StatefulSet: %v", err)
				}
				containers = s.Spec.Template.Spec.Containers
				annotations = s.Spec.Template.Annotations

			default:
				// For non-Deployment/StatefulSet, no changes expected
				return
			}

			// Verify env vars for each container
			for _, container := range containers {
				wantEnvVars := tt.wantEnvVars[container.Name]
				gotEnvVars := container.Env

				if diff := cmp.Diff(wantEnvVars, gotEnvVars); diff != "" {
					t.Errorf("Container %q env vars mismatch (-want +got):\n%s", container.Name, diff)
				}
			}

			// Verify annotation
			gotAnnotation := annotations["operator.tekton.dev/tls-config-hash"]
			if gotAnnotation != tt.wantAnnotation {
				t.Errorf("Pod template annotation = %v, want %v", gotAnnotation, tt.wantAnnotation)
			}
		})
	}
}

func TestInjectTLSEnvVarsTransformer_NonTargetResources(t *testing.T) {
	// Setup context with TLS config
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
	ctx := context.Background()
	ctx = context.WithValue(ctx, tlsConfigKey{}, tlsConfig)
	ctx = context.WithValue(ctx, tlsConfigHashKey{}, "test-hash")

	// Test resources that should NOT be modified
	tests := []struct {
		name     string
		resource *unstructured.Unstructured
	}{
		{
			name: "Service is not modified",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"name": "test-service",
					},
				},
			},
		},
		{
			name: "ConfigMap is not modified",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "test-configmap",
					},
				},
			},
		},
		{
			name: "Pod is not modified",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "test-pod",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get original resource
			originalJSON, _ := tt.resource.MarshalJSON()

			// Apply transformer
			transformer := InjectTLSEnvVarsTransformer(ctx)
			err := transformer(tt.resource)
			if err != nil {
				t.Fatalf("InjectTLSEnvVarsTransformer() error = %v", err)
			}

			// Verify resource was not modified
			afterJSON, _ := tt.resource.MarshalJSON()
			if string(originalJSON) != string(afterJSON) {
				t.Errorf("Resource was modified but should not have been")
			}
		})
	}
}

func TestMergeEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1.EnvVar
		new      []corev1.EnvVar
		want     []corev1.EnvVar
	}{
		{
			name:     "Empty existing, add new",
			existing: []corev1.EnvVar{},
			new: []corev1.EnvVar{
				{Name: "VAR1", Value: "value1"},
			},
			want: []corev1.EnvVar{
				{Name: "VAR1", Value: "value1"},
			},
		},
		{
			name: "Existing vars preserved, new vars added",
			existing: []corev1.EnvVar{
				{Name: "EXISTING", Value: "existing-value"},
			},
			new: []corev1.EnvVar{
				{Name: "NEW", Value: "new-value"},
			},
			want: []corev1.EnvVar{
				{Name: "EXISTING", Value: "existing-value"},
				{Name: "NEW", Value: "new-value"},
			},
		},
		{
			name: "Existing vars replaced with new values",
			existing: []corev1.EnvVar{
				{Name: "VAR1", Value: "old-value"},
			},
			new: []corev1.EnvVar{
				{Name: "VAR1", Value: "new-value"},
			},
			want: []corev1.EnvVar{
				{Name: "VAR1", Value: "new-value"},
			},
		},
		{
			name: "Mixed: some replaced, some added",
			existing: []corev1.EnvVar{
				{Name: "VAR1", Value: "old-value1"},
				{Name: "VAR2", Value: "value2"},
			},
			new: []corev1.EnvVar{
				{Name: "VAR1", Value: "new-value1"},
				{Name: "VAR3", Value: "value3"},
			},
			want: []corev1.EnvVar{
				{Name: "VAR1", Value: "new-value1"}, // Replaced
				{Name: "VAR2", Value: "value2"},     // Preserved
				{Name: "VAR3", Value: "value3"},     // Added
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeEnvVars(tt.existing, tt.new)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mergeEnvVars() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Helper function to convert typed objects to unstructured
func mustToUnstructured(obj interface{}) *unstructured.Unstructured {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: unstructuredObj}
}

// TestInjectTLSEnvVarsTransformer_RealWorld tests with a more realistic deployment
func TestInjectTLSEnvVarsTransformer_RealWorld(t *testing.T) {
	// Setup TLS config similar to what we'd see in production
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
		},
	}
	tlsHash := CalculateTLSConfigHash(tlsConfig)

	ctx := context.Background()
	ctx = context.WithValue(ctx, tlsConfigKey{}, tlsConfig)
	ctx = context.WithValue(ctx, tlsConfigHashKey{}, tlsHash)

	// Create a deployment with multiple containers and existing env vars
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-results-api",
			Namespace: "openshift-pipelines",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: "registry.redhat.io/openshift-pipelines/tekton-results-api:v1.0",
							Env: []corev1.EnvVar{
								{Name: "DB_HOST", Value: "tekton-results-postgres"},
								{Name: "DB_PORT", Value: "5432"},
							},
						},
						{
							Name:  "watcher",
							Image: "registry.redhat.io/openshift-pipelines/tekton-results-watcher:v1.0",
						},
					},
				},
			},
		},
	}

	u := mustToUnstructured(deployment)

	// Apply transformer - inject only into "api" container
	transformer := InjectTLSEnvVarsTransformer(ctx, "api")
	err := transformer(u)
	if err != nil {
		t.Fatalf("InjectTLSEnvVarsTransformer() error = %v", err)
	}

	// Convert back
	result := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, result)
	if err != nil {
		t.Fatalf("Failed to convert result: %v", err)
	}

	// Verify "api" container has TLS env vars
	apiContainer := result.Spec.Template.Spec.Containers[0]
	if apiContainer.Name != "api" {
		t.Fatalf("Expected first container to be 'api', got %q", apiContainer.Name)
	}

	// Should have original env vars + TLS env vars
	expectedEnvVars := []corev1.EnvVar{
		{Name: "DB_HOST", Value: "tekton-results-postgres"},
		{Name: "DB_PORT", Value: "5432"},
		{Name: TLSMinVersionEnvVar, Value: "TLSv1.2"},
		{Name: TLSCipherSuitesEnvVar, Value: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
		{Name: TLSCurvePreferencesEnvVar, Value: "P-256,P-384"},
	}

	if diff := cmp.Diff(expectedEnvVars, apiContainer.Env); diff != "" {
		t.Errorf("API container env vars mismatch (-want +got):\n%s", diff)
	}

	// Verify "watcher" container does NOT have TLS env vars
	watcherContainer := result.Spec.Template.Spec.Containers[1]
	if watcherContainer.Name != "watcher" {
		t.Fatalf("Expected second container to be 'watcher', got %q", watcherContainer.Name)
	}

	if len(watcherContainer.Env) != 0 {
		t.Errorf("Watcher container should have no env vars, got %v", watcherContainer.Env)
	}

	// Verify annotations
	if result.Spec.Template.Annotations["existing-annotation"] != "existing-value" {
		t.Error("Existing annotation was not preserved")
	}

	if result.Spec.Template.Annotations["operator.tekton.dev/tls-config-hash"] != tlsHash {
		t.Errorf("TLS hash annotation = %v, want %v",
			result.Spec.Template.Annotations["operator.tekton.dev/tls-config-hash"],
			tlsHash)
	}
}
