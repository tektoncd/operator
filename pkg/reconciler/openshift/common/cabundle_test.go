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
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestApplyCABundles(t *testing.T) {
	actual := unstructuredDeployment(t)
	expected := unstructuredDeployment(t,
		withEnvs(
			corev1.EnvVar{
				Name:  "SSL_CERT_DIR",
				Value: "/tekton-custom-certs:/etc/ssl/certs:/etc/pki/tls/certs:/system/etc/security/cacerts",
			},
		),
		withVolumes(corev1.Volume{
			Name: common.TrustedCAConfigMapVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: common.TrustedCAConfigMapName},
					Items: []corev1.KeyToPath{
						{
							Key:  common.TrustedCAKey,
							Path: common.TrustedCAKey,
						},
					},
				},
			},
		},
			corev1.Volume{
				Name: common.ServiceCAConfigMapVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: common.ServiceCAConfigMapName},
						Items: []corev1.KeyToPath{
							{
								Key:  common.ServiceCAKey,
								Path: common.ServiceCAKey,
							},
						},
					},
				},
			}),
		withVolumeMounts(
			corev1.VolumeMount{
				Name:      common.TrustedCAConfigMapVolume,
				MountPath: filepath.Join("/tekton-custom-certs", common.TrustedCAKey),
				SubPath:   common.TrustedCAKey,
				ReadOnly:  true,
			},
			corev1.VolumeMount{
				Name:      common.ServiceCAConfigMapVolume,
				MountPath: filepath.Join("/tekton-custom-certs", common.ServiceCAKey),
				SubPath:   common.ServiceCAKey,
				ReadOnly:  true,
			},
		),
	)

	if err := ApplyCABundles(actual); err != nil {
		t.Fatal(err)
	}

	assert.DeepEqual(t, actual, expected)
}

type deploymentModifier func(*appsv1.Deployment)

func unstructuredDeployment(t *testing.T, modifiers ...deploymentModifier) *unstructured.Unstructured {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "registry",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "registry",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "registry",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "registry",
						Image: "registry",
					}},
				},
			},
		},
	}

	for _, modifier := range modifiers {
		modifier(deploy)
	}

	deploy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   appsv1.SchemeGroupVersion.Group,
		Version: appsv1.SchemeGroupVersion.Version,
		Kind:    "Deployment",
	})
	b, err := json.Marshal(deploy)
	if err != nil {
		t.Fatal(err)
	}
	ud := &unstructured.Unstructured{}
	if err := json.Unmarshal(b, ud); err != nil {
		t.Fatal(err)
	}
	return ud
}

func withEnvs(envs ...corev1.EnvVar) func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		for i, c := range d.Spec.Template.Spec.Containers {
			c.Env = append(c.Env, envs...)
			d.Spec.Template.Spec.Containers[i] = c
		}
	}
}

func withVolumes(volumes ...corev1.Volume) func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, volumes...)
	}
}

func withVolumeMounts(volumeMounts ...corev1.VolumeMount) func(*appsv1.Deployment) {
	return func(d *appsv1.Deployment) {
		for i, c := range d.Spec.Template.Spec.Containers {
			c.VolumeMounts = append(c.VolumeMounts, volumeMounts...)
			d.Spec.Template.Spec.Containers[i] = c
		}
	}
}
