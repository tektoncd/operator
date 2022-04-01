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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	// user-provided and system CA certificates
	TrustedCAConfigMapName   = "config-trusted-cabundle"
	TrustedCAConfigMapVolume = "config-trusted-cabundle-volume"
	TrustedCAKey             = "ca-bundle.crt"

	// service serving certificates (required to talk to the internal registry)
	ServiceCAConfigMapName   = "config-service-cabundle"
	ServiceCAConfigMapVolume = "config-service-cabundle-volume"
	ServiceCAKey             = "service-ca.crt"
)

// newVolumeWithConfigMap creates a new volume with the given ConfigMap
func newVolumeWithConfigMap(volumeName, configMapName, configMapKey, configMapPath string) corev1.Volume {
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				Items: []corev1.KeyToPath{
					{
						Key:  configMapKey,
						Path: configMapPath,
					},
				},
			},
		},
	}
}

// AddCABundleConfigMapsToVolumes adds the config-trusted-cabundle and config-service-cabundle
// ConfigMaps to the given list of volumes and removes duplicates, if any
func AddCABundleConfigMapsToVolumes(volumes []corev1.Volume) []corev1.Volume {
	// If CA bundle volumes already exists in the pod's volumes, then remove it
	for _, volumeName := range []string{TrustedCAConfigMapVolume, ServiceCAConfigMapVolume} {
		for i, v := range volumes {
			if v.Name == volumeName {
				volumes = append(volumes[:i], volumes[i+1:]...)
				break
			}
		}
	}

	return append(
		volumes,
		newVolumeWithConfigMap(TrustedCAConfigMapVolume, TrustedCAConfigMapName, TrustedCAKey, TrustedCAKey),
		newVolumeWithConfigMap(ServiceCAConfigMapVolume, ServiceCAConfigMapName, ServiceCAKey, ServiceCAKey),
	)
}

// AddCABundlesToContainerVolumes adds the CA bundles to the container via VolumeMounts.
// SSL_CERT_DIR environment variable is also set if it does not exist already.
func AddCABundlesToContainerVolumes(c *corev1.Container) {
	volumeMounts := c.VolumeMounts

	// If volume mounts for CA bundles already exist then remove them
	for _, volumeName := range []string{TrustedCAConfigMapVolume, ServiceCAConfigMapVolume} {
		for i, vm := range volumeMounts {
			if vm.Name == volumeName {
				volumeMounts = append(volumeMounts[:i], volumeMounts[i+1:]...)
				break
			}
		}
	}

	// We will mount the certs at /tekton-custom-certs so we don't override the existing certs
	sslCertDir := "/tekton-custom-certs"
	certEnvAvailable := false

	for _, env := range c.Env {
		// If SSL_CERT_DIR env var already exists, then we don't mess with
		// it and simply carry it forward as it is
		if env.Name == "SSL_CERT_DIR" {
			sslCertDir = env.Value
			certEnvAvailable = true
			break
		}
	}

	if !certEnvAvailable {
		// Here, we need to set the default value for SSL_CERT_DIR.
		// Keep in mind that if SSL_CERT_DIR is set, then it overrides the
		// system default, i.e. the system default directories will "NOT"
		// be scanned for certificates. This is risky and we don't want to
		// do this because users mount certificates at these locations or
		// build images with certificates "in" them and expect certificates
		// to get picked up, and rightfully so since this is the documented
		// way of achieving this.
		// So, let's keep the system wide default locations in place and
		// "append" our custom location to those.
		//
		// certDirectories copied from
		// https://golang.org/src/crypto/x509/root_linux.go
		var certDirectories = []string{
			// Ordering is important here - we will be using the "first"
			// element in SSL_CERT_DIR to do the volume mounts.
			sslCertDir,                     // /tekton-custom-certs
			"/etc/ssl/certs",               // SLES10/SLES11, https://golang.org/issue/12139
			"/etc/pki/tls/certs",           // Fedora/RHEL
			"/system/etc/security/cacerts", // Android
		}

		// SSL_CERT_DIR accepts a colon separated list of directories
		sslCertDir = strings.Join(certDirectories, ":")
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  "SSL_CERT_DIR",
			Value: sslCertDir,
		})
	}

	// Let's mount the certificates now.
	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name: TrustedCAConfigMapVolume,
			// We only want the first entry in SSL_CERT_DIR for the mount
			MountPath: filepath.Join(strings.Split(sslCertDir, ":")[0], TrustedCAKey),
			SubPath:   TrustedCAKey,
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name: ServiceCAConfigMapVolume,
			// We only want the first entry in SSL_CERT_DIR for the mount
			MountPath: filepath.Join(strings.Split(sslCertDir, ":")[0], ServiceCAKey),
			SubPath:   ServiceCAKey,
			ReadOnly:  true,
		},
	)
	c.VolumeMounts = volumeMounts
}
