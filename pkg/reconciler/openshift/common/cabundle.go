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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	volumeName     = "config-registry-cert" // override default one
	cfgMapName     = "config-trusted-cabundle"
	cfgMapItemKey  = "ca-bundle.crt"
	cfgMapItemPath = "tls-ca-bundle.pem"
)

// ApplyTrustedCABundle is a transformer that add the trustedCA volume, mount and
// environment variables so that the deployment uses it.
func ApplyTrustedCABundle(u *unstructured.Unstructured) error {
	if u.GetKind() != "Deployment" {
		// Don't do anything on something else than Deployment
		return nil
	}

	deployment := &appsv1.Deployment{}
	if err := scheme.Scheme.Convert(u, deployment, nil); err != nil {
		return err
	}

	volumes := deployment.Spec.Template.Spec.Volumes
	for i, v := range volumes {
		if v.Name == volumeName {
			volumes = append(volumes[:i], volumes[i+1:]...)
			break
		}
	}
	volumes = append(volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cfgMapName},
				Items: []corev1.KeyToPath{{
					Key:  cfgMapItemKey,
					Path: cfgMapItemPath,
				}},
			},
		},
	})
	deployment.Spec.Template.Spec.Volumes = volumes

	for i, c := range deployment.Spec.Template.Spec.Containers {
		volumeMounts := c.VolumeMounts
		for i, vm := range volumeMounts {
			if vm.Name == volumeName {
				volumeMounts = append(volumeMounts[:i], volumeMounts[i+1:]...)
				break
			}
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/etc/config-registry-cert/",
			ReadOnly:  true,
		})
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
