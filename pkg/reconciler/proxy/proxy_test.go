/*
Copyright 2020 The Tekton Authors

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

package proxy

import (
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
)

func TestUpdateVolume(t *testing.T) {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{},
			Containers: []v1.Container{
				{
					Name:  "testc",
					Image: "testi",
				},
			},
		},
	}
	// Test the new optional approach that doesn't require API calls
	podUpdated := updateVolumeOptional(pod)
	assert.DeepEqual(t, len(podUpdated.Spec.Containers[0].Env), 1)
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].Env[0].Name, "SSL_CERT_DIR")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].Env[0].Value, "/tekton-custom-certs:/etc/ssl/certs:/etc/pki/tls/certs")

	assert.DeepEqual(t, len(podUpdated.Spec.Volumes), 2)
	assert.DeepEqual(t, podUpdated.Spec.Volumes[0].Name, "config-trusted-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Volumes[0].ConfigMap.Name, "config-trusted-cabundle")
	assert.Assert(t, podUpdated.Spec.Volumes[0].ConfigMap.Optional != nil)
	assert.DeepEqual(t, *podUpdated.Spec.Volumes[0].ConfigMap.Optional, true)

	assert.DeepEqual(t, podUpdated.Spec.Volumes[1].Name, "config-service-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Volumes[1].ConfigMap.Name, "config-service-cabundle")
	assert.Assert(t, podUpdated.Spec.Volumes[1].ConfigMap.Optional != nil)
	assert.DeepEqual(t, *podUpdated.Spec.Volumes[1].ConfigMap.Optional, true)

	assert.DeepEqual(t, len(podUpdated.Spec.Containers[0].VolumeMounts), 2)
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[0].Name, "config-trusted-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[0].SubPath, "ca-bundle.crt")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[1].Name, "config-service-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[1].SubPath, "service-ca.crt")
}

// TestUpdateVolumeOptionalWithExistingVolumes tests that optional volumes replace existing ones correctly
func TestUpdateVolumeOptionalWithExistingVolumes(t *testing.T) {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "config-trusted-cabundle-volume",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{Name: "old-config"},
						},
					},
				},
				{
					Name: "other-volume",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:  "testc",
					Image: "testi",
				},
			},
		},
	}
	podUpdated := updateVolumeOptional(pod)

	// Should have 3 volumes: other-volume + 2 CA bundle volumes
	assert.DeepEqual(t, len(podUpdated.Spec.Volumes), 3)

	// Check that old volume is preserved
	var otherVolumeFound bool
	for _, vol := range podUpdated.Spec.Volumes {
		if vol.Name == "other-volume" {
			otherVolumeFound = true
			break
		}
	}
	assert.Assert(t, otherVolumeFound, "other-volume should be preserved")

	// Check that CA bundle volumes are properly configured with Optional=true
	var trustedVolumeFound, serviceVolumeFound bool
	for _, vol := range podUpdated.Spec.Volumes {
		switch vol.Name {
		case "config-trusted-cabundle-volume":
			trustedVolumeFound = true
			assert.DeepEqual(t, vol.ConfigMap.Name, "config-trusted-cabundle")
			assert.Assert(t, vol.ConfigMap.Optional != nil)
			assert.DeepEqual(t, *vol.ConfigMap.Optional, true)
		case "config-service-cabundle-volume":
			serviceVolumeFound = true
			assert.DeepEqual(t, vol.ConfigMap.Name, "config-service-cabundle")
			assert.Assert(t, vol.ConfigMap.Optional != nil)
			assert.DeepEqual(t, *vol.ConfigMap.Optional, true)
		}
	}
	assert.Assert(t, trustedVolumeFound, "trusted CA volume should be present")
	assert.Assert(t, serviceVolumeFound, "service CA volume should be present")
}

// TestUpdateVolumeOptionalEmptyConfigMaps tests behavior when ConfigMaps don't exist
// With Optional=true, pods should start successfully even with missing ConfigMaps
func TestUpdateVolumeOptionalEmptyConfigMaps(t *testing.T) {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{},
			Containers: []v1.Container{
				{
					Name:  "testc",
					Image: "testi",
					Env: []v1.EnvVar{
						{
							Name:  "EXISTING_ENV",
							Value: "existing_value",
						},
					},
				},
			},
		},
	}
	podUpdated := updateVolumeOptional(pod)

	// Environment variables should be preserved and SSL_CERT_DIR added
	assert.DeepEqual(t, len(podUpdated.Spec.Containers[0].Env), 2)

	// Check existing env var is preserved
	var existingEnvFound bool
	for _, env := range podUpdated.Spec.Containers[0].Env {
		if env.Name == "EXISTING_ENV" {
			existingEnvFound = true
			assert.DeepEqual(t, env.Value, "existing_value")
			break
		}
	}
	assert.Assert(t, existingEnvFound, "existing environment variable should be preserved")

	// Check SSL_CERT_DIR is added
	var sslCertDirFound bool
	for _, env := range podUpdated.Spec.Containers[0].Env {
		if env.Name == "SSL_CERT_DIR" {
			sslCertDirFound = true
			assert.DeepEqual(t, env.Value, "/tekton-custom-certs:/etc/ssl/certs:/etc/pki/tls/certs")
			break
		}
	}
	assert.Assert(t, sslCertDirFound, "SSL_CERT_DIR should be added")

	// Volumes should still be added with Optional=true (this allows graceful handling of missing ConfigMaps)
	assert.DeepEqual(t, len(podUpdated.Spec.Volumes), 2)
	for _, vol := range podUpdated.Spec.Volumes {
		assert.Assert(t, vol.ConfigMap.Optional != nil, "ConfigMap volume should be optional")
		assert.DeepEqual(t, *vol.ConfigMap.Optional, true)
	}

	// Volume mounts should be added (they won't fail with optional=true even if ConfigMap is missing)
	assert.DeepEqual(t, len(podUpdated.Spec.Containers[0].VolumeMounts), 2)
}
