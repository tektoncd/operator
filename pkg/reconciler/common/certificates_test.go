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

package common

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"
)

func TestAddCABundleConfigMapsToVolumes(t *testing.T) {
	type testStructure struct {
		name     string
		input    []corev1.Volume
		expected []corev1.Volume
	}

	tests := []testStructure{
		{
			name:  "Vanilla test without any input volumes",
			input: nil,
			expected: []corev1.Volume{
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  TrustedCAKey,
									Path: TrustedCAKey,
								},
							},
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  ServiceCAKey,
									Path: ServiceCAKey,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Check if volumes are appended",
			input: []corev1.Volume{
				{
					Name: "bleh",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "bleh"},
							Items: []corev1.KeyToPath{
								{
									Key:  "bleh",
									Path: "bleh",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Volume{
				{
					Name: "bleh",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "bleh"},
							Items: []corev1.KeyToPath{
								{
									Key:  "bleh",
									Path: "bleh",
								},
							},
						},
					},
				},
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  TrustedCAKey,
									Path: TrustedCAKey,
								},
							},
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  ServiceCAKey,
									Path: ServiceCAKey,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Check if duplicate volumes are removed",
			input: []corev1.Volume{
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "bleh"},
							Items: []corev1.KeyToPath{
								{
									Key:  "bleh",
									Path: "bleh",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Volume{
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  TrustedCAKey,
									Path: TrustedCAKey,
								},
							},
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  ServiceCAKey,
									Path: ServiceCAKey,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("Running test: %v", test.name)
		actualOutput := AddCABundleConfigMapsToVolumes(test.input)
		assert.DeepEqual(t, actualOutput, test.expected)
	}
}

func TestAddCABundlesToContainerVolumes(t *testing.T) {
	type testStructure struct {
		name     string
		input    *corev1.Container
		expected *corev1.Container
	}

	defaultSSLCertDir := "/tekton-custom-certs:/etc/ssl/certs:/etc/pki/tls/certs"

	tests := []testStructure{
		{
			name:  "Check baseline functionality - default SSL_CERT_DIR value, default volume mounts",
			input: &corev1.Container{},
			expected: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "SSL_CERT_DIR",
						Value: defaultSSLCertDir,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      TrustedCAConfigMapVolume,
						MountPath: filepath.Join("/tekton-custom-certs", TrustedCAKey),
						SubPath:   TrustedCAKey,
						ReadOnly:  true,
					},
					{
						Name:      ServiceCAConfigMapVolume,
						MountPath: filepath.Join("/tekton-custom-certs", ServiceCAKey),
						SubPath:   ServiceCAKey,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "Check if duplicates are removed",
			input: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      TrustedCAConfigMapVolume,
						MountPath: "bleh",
						SubPath:   "bleh",
						ReadOnly:  false,
					},
				},
			},
			expected: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "SSL_CERT_DIR",
						Value: defaultSSLCertDir,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      TrustedCAConfigMapVolume,
						MountPath: filepath.Join("/tekton-custom-certs", TrustedCAKey),
						SubPath:   TrustedCAKey,
						ReadOnly:  true,
					},
					{
						Name:      ServiceCAConfigMapVolume,
						MountPath: filepath.Join("/tekton-custom-certs", ServiceCAKey),
						SubPath:   ServiceCAKey,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "Check if volume mounts are appended",
			input: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "bleh",
						MountPath: "bleh",
						SubPath:   "bleh",
						ReadOnly:  false,
					},
				},
			},
			expected: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "SSL_CERT_DIR",
						Value: defaultSSLCertDir,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "bleh",
						MountPath: "bleh",
						SubPath:   "bleh",
						ReadOnly:  false,
					},
					{
						Name:      TrustedCAConfigMapVolume,
						MountPath: filepath.Join("/tekton-custom-certs", TrustedCAKey),
						SubPath:   TrustedCAKey,
						ReadOnly:  true,
					},
					{
						Name:      ServiceCAConfigMapVolume,
						MountPath: filepath.Join("/tekton-custom-certs", ServiceCAKey),
						SubPath:   ServiceCAKey,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "Check if already existing SSL_CERT_DIR is preserved",
			input: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "SSL_CERT_DIR",
						Value: "/existing/ssl/cert/dir",
					},
				},
			},
			expected: &corev1.Container{
				Env: []corev1.EnvVar{
					{
						Name:  "SSL_CERT_DIR",
						Value: "/existing/ssl/cert/dir",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      TrustedCAConfigMapVolume,
						MountPath: filepath.Join("/existing/ssl/cert/dir", TrustedCAKey),
						SubPath:   TrustedCAKey,
						ReadOnly:  true,
					},
					{
						Name:      ServiceCAConfigMapVolume,
						MountPath: filepath.Join("/existing/ssl/cert/dir", ServiceCAKey),
						SubPath:   ServiceCAKey,
						ReadOnly:  true,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("Running test: %v", test.name)
		AddCABundlesToContainerVolumes(test.input)
		assert.DeepEqual(t, test.input, test.expected)
	}
}

// TestNewVolumeWithConfigMapOptional tests the new function that creates optional ConfigMap volumes
func TestNewVolumeWithConfigMapOptional(t *testing.T) {
	type testStructure struct {
		name          string
		volumeName    string
		configMapName string
		configMapKey  string
		configMapPath string
		expected      corev1.Volume
	}

	tests := []testStructure{
		{
			name:          "Basic optional ConfigMap volume creation",
			volumeName:    "test-volume",
			configMapName: "test-configmap",
			configMapKey:  "test-key",
			configMapPath: "test-path",
			expected: corev1.Volume{
				Name: "test-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "test-configmap"},
						Items: []corev1.KeyToPath{
							{
								Key:  "test-key",
								Path: "test-path",
							},
						},
						Optional: ptr.Bool(true),
					},
				},
			},
		},
		{
			name:          "Trusted CA bundle volume with optional",
			volumeName:    TrustedCAConfigMapVolume,
			configMapName: TrustedCAConfigMapName,
			configMapKey:  TrustedCAKey,
			configMapPath: TrustedCAKey,
			expected: corev1.Volume{
				Name: TrustedCAConfigMapVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
						Items: []corev1.KeyToPath{
							{
								Key:  TrustedCAKey,
								Path: TrustedCAKey,
							},
						},
						Optional: ptr.Bool(true),
					},
				},
			},
		},
		{
			name:          "Service CA bundle volume with optional",
			volumeName:    ServiceCAConfigMapVolume,
			configMapName: ServiceCAConfigMapName,
			configMapKey:  ServiceCAKey,
			configMapPath: ServiceCAKey,
			expected: corev1.Volume{
				Name: ServiceCAConfigMapVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
						Items: []corev1.KeyToPath{
							{
								Key:  ServiceCAKey,
								Path: ServiceCAKey,
							},
						},
						Optional: ptr.Bool(true),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("Running test: %v", test.name)
		actualOutput := NewVolumeWithConfigMapOptional(test.volumeName, test.configMapName, test.configMapKey, test.configMapPath)
		assert.DeepEqual(t, actualOutput, test.expected)

		// Verify that Optional is specifically set to true
		assert.Assert(t, actualOutput.ConfigMap.Optional != nil, "Optional field should be set")
		assert.DeepEqual(t, *actualOutput.ConfigMap.Optional, true)
	}
}

// TestAddCABundleConfigMapsToVolumesOptional tests the new function that adds optional CA bundle volumes
func TestAddCABundleConfigMapsToVolumesOptional(t *testing.T) {
	type testStructure struct {
		name     string
		input    []corev1.Volume
		expected []corev1.Volume
	}

	tests := []testStructure{
		{
			name:  "Vanilla test without any input volumes - optional variant",
			input: nil,
			expected: []corev1.Volume{
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  TrustedCAKey,
									Path: TrustedCAKey,
								},
							},
							Optional: ptr.Bool(true),
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  ServiceCAKey,
									Path: ServiceCAKey,
								},
							},
							Optional: ptr.Bool(true),
						},
					},
				},
			},
		},
		{
			name: "Check if volumes are appended - optional variant",
			input: []corev1.Volume{
				{
					Name: "existing-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "existing-configmap"},
							Items: []corev1.KeyToPath{
								{
									Key:  "existing-key",
									Path: "existing-path",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Volume{
				{
					Name: "existing-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "existing-configmap"},
							Items: []corev1.KeyToPath{
								{
									Key:  "existing-key",
									Path: "existing-path",
								},
							},
						},
					},
				},
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  TrustedCAKey,
									Path: TrustedCAKey,
								},
							},
							Optional: ptr.Bool(true),
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  ServiceCAKey,
									Path: ServiceCAKey,
								},
							},
							Optional: ptr.Bool(true),
						},
					},
				},
			},
		},
		{
			name: "Check if duplicate volumes are removed - optional variant",
			input: []corev1.Volume{
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "old-config"},
							Items: []corev1.KeyToPath{
								{
									Key:  "old-key",
									Path: "old-path",
								},
							},
							// Note: old volume might not have Optional set
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "old-service-config"},
							Items: []corev1.KeyToPath{
								{
									Key:  "old-service-key",
									Path: "old-service-path",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Volume{
				{
					Name: TrustedCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: TrustedCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  TrustedCAKey,
									Path: TrustedCAKey,
								},
							},
							Optional: ptr.Bool(true),
						},
					},
				},
				{
					Name: ServiceCAConfigMapVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ServiceCAConfigMapName},
							Items: []corev1.KeyToPath{
								{
									Key:  ServiceCAKey,
									Path: ServiceCAKey,
								},
							},
							Optional: ptr.Bool(true),
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Logf("Running test: %v", test.name)
		actualOutput := AddCABundleConfigMapsToVolumesOptional(test.input)
		assert.DeepEqual(t, actualOutput, test.expected)

		// Verify that all CA bundle volumes have Optional=true
		for _, volume := range actualOutput {
			if volume.Name == TrustedCAConfigMapVolume || volume.Name == ServiceCAConfigMapVolume {
				assert.Assert(t, volume.ConfigMap.Optional != nil, "CA bundle volume should have Optional field set")
				assert.DeepEqual(t, *volume.ConfigMap.Optional, true)
			}
		}
	}
}
