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

package common

import (
	"encoding/json"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	// user-provided and system CA certificates
	trustedCAConfigMapName   = "config-trusted-cabundle"
	trustedCAConfigMapVolume = "config-trusted-cabundle-volume"
	trustedCAKey             = "ca-bundle.crt"

	// service serving certificates (required to talk to the internal registry)
	serviceCAConfigMapName   = "config-service-cabundle"
	serviceCAConfigMapVolume = "config-service-cabundle-volume"
	serviceCAKey             = "service-ca.crt"
)

// ApplyCABundles is a transformer that add the trustedCA volume, mount and
// environment variables so that the deployment uses it.
func ApplyCABundles(u *unstructured.Unstructured) error {
	if u.GetKind() != "Deployment" {
		// Don't do anything on something else than Deployment
		return nil
	}

	deployment := &appsv1.Deployment{}
	if err := scheme.Scheme.Convert(u, deployment, nil); err != nil {
		return err
	}

	volumes := deployment.Spec.Template.Spec.Volumes

	// If CA bundle volumes already exists in the PodSpec, then remove it
	for _, volumeName := range []string{trustedCAConfigMapVolume, serviceCAConfigMapVolume} {
		for i, v := range volumes {
			if v.Name == volumeName {
				volumes = append(volumes[:i], volumes[i+1:]...)
				break
			}
		}
	}

	// Let's add the trusted and service CA bundle ConfigMaps as a volume in
	// the PodSpec which will later be mounted to add certs in the pod.
	volumes = append(volumes,
		// Add trusted CA bundle
		corev1.Volume{
			Name: trustedCAConfigMapVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: trustedCAConfigMapName},
					Items: []corev1.KeyToPath{
						{
							Key:  trustedCAKey,
							Path: trustedCAKey,
						},
					},
				},
			},
		},
		// Add service serving certificates bundle
		corev1.Volume{
			Name: serviceCAConfigMapVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: serviceCAConfigMapName},
					Items: []corev1.KeyToPath{
						{
							Key:  serviceCAKey,
							Path: serviceCAKey,
						},
					},
				},
			},
		})
	deployment.Spec.Template.Spec.Volumes = volumes

	// Now that the injected certificates have been added as a volume, let's
	// mount them via volumeMounts in the containers
	for i, c := range deployment.Spec.Template.Spec.Containers {
		volumeMounts := c.VolumeMounts

		// If volume mounts for injected certificates already exist then remove them
		for _, volumeName := range []string{trustedCAConfigMapVolume, serviceCAConfigMapVolume} {
			for i, vm := range volumeMounts {
				if vm.Name == volumeName {
					volumeMounts = append(volumeMounts[:i], volumeMounts[i+1:]...)
					break
				}
			}
		}

		// We will mount the certs at this location so we don't override the existing certs
		sslCertDir := "/tekton-custom-certs"
		for _, env := range c.Env {
			if env.Name == "SSL_CERT_DIR" {
				sslCertDir = env.Value
			}
		}

		// Let's mount the certificates now.
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name: trustedCAConfigMapVolume,
				// We only want the first entry in SSL_CERT_DIR for the mount
				MountPath: filepath.Join(strings.Split(sslCertDir, ":")[0], trustedCAKey),
				SubPath:   trustedCAKey,
				ReadOnly:  true,
			},
			corev1.VolumeMount{
				Name: serviceCAConfigMapVolume,
				// We only want the first entry in SSL_CERT_DIR for the mount
				MountPath: filepath.Join(strings.Split(sslCertDir, ":")[0], serviceCAKey),
				SubPath:   serviceCAKey,
				ReadOnly:  true,
			},
		)
		c.VolumeMounts = volumeMounts
		deployment.Spec.Template.Spec.Containers[i] = c
	}

	deployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   appsv1.SchemeGroupVersion.Group,
		Version: appsv1.SchemeGroupVersion.Version,
		Kind:    "Deployment",
	})
	m, err := toUnstructured(deployment)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(m.Object)
	return nil
}

func toUnstructured(v interface{}) (*unstructured.Unstructured, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	ud := &unstructured.Unstructured{}
	if err := json.Unmarshal(b, ud); err != nil {
		return nil, err
	}
	return ud, nil
}
