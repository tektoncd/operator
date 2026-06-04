/*
Copyright 2026 The Tekton Authors

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
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

// ---------------------------------------------------------------------------
// replaceServerNameNamespace
// ---------------------------------------------------------------------------

func TestReplaceServerNameNamespace(t *testing.T) {
	tests := []struct {
		name            string
		serverName      string
		targetNamespace string
		want            string
	}{
		{
			name:            "simple svc name",
			serverName:      "tekton-pipelines-controller.tekton-pipelines.svc",
			targetNamespace: "openshift-pipelines",
			want:            "tekton-pipelines-controller.openshift-pipelines.svc",
		},
		{
			name:            "with cluster.local suffix",
			serverName:      "foo.bar.svc.cluster.local",
			targetNamespace: "my-ns",
			want:            "foo.my-ns.svc.cluster.local",
		},
		{
			name:            "no .svc segment returns unchanged",
			serverName:      "foo.bar.example.com",
			targetNamespace: "my-ns",
			want:            "foo.bar.example.com",
		},
		{
			name:            "missing namespace segment returns unchanged",
			serverName:      "foo.svc",
			targetNamespace: "my-ns",
			// "foo.svc" → base="foo", SplitN gives only 1 part → unchanged
			want: "foo.svc",
		},
		{
			name:            "already correct namespace is replaced",
			serverName:      "svc-a.target-ns.svc",
			targetNamespace: "target-ns",
			want:            "svc-a.target-ns.svc",
		},
		{
			name:            "empty string is unchanged",
			serverName:      "",
			targetNamespace: "my-ns",
			want:            "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := replaceServerNameNamespace(tc.serverName, tc.targetNamespace)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyMetricsTLS — Deployment
// ---------------------------------------------------------------------------

func TestApplyMetricsTLSDeployment(t *testing.T) {
	const secretName = "tekton-controller-metrics-tls"
	ud := unstructuredDeployment(t) // name="registry", one container

	assert.NilError(t, ApplyMetricsTLS("Deployment", "registry", secretName)(ud))

	d := &appsv1.Deployment{}
	assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, d))

	spec := d.Spec.Template.Spec

	// Two volumes must be present.
	assertVolumeSecret(t, spec.Volumes, metricsServingCertVolume, secretName)
	assertVolumeConfigMap(t, spec.Volumes, metricsClientCAVolume, MetricsClientCAConfigMap)

	// Every container must have the mounts and env vars.
	for _, c := range spec.Containers {
		assertVolumeMount(t, c.VolumeMounts, metricsServingCertVolume, MetricsTLSMountPath)
		assertVolumeMount(t, c.VolumeMounts, metricsClientCAVolume, MetricsClientCAMountPath)
		assertEnv(t, c.Env, envMetricsTLSCert, fmt.Sprintf("%s/tls.crt", MetricsTLSMountPath))
		assertEnv(t, c.Env, envMetricsTLSKey, fmt.Sprintf("%s/tls.key", MetricsTLSMountPath))
		assertEnv(t, c.Env, envMetricsTLSClientCA, fmt.Sprintf("%s/%s", MetricsClientCAMountPath, MetricsClientCAKey))
		assertEnv(t, c.Env, envMetricsTLSClientAuth, "require")
	}
}

func TestApplyMetricsTLSDeploymentIdempotent(t *testing.T) {
	const secretName = "tekton-controller-metrics-tls"
	ud := unstructuredDeployment(t)

	assert.NilError(t, ApplyMetricsTLS("Deployment", "registry", secretName)(ud))
	// Apply a second time — result must be identical (no duplicates).
	snapshot := ud.DeepCopy()
	assert.NilError(t, ApplyMetricsTLS("Deployment", "registry", secretName)(ud))

	d1 := &appsv1.Deployment{}
	d2 := &appsv1.Deployment{}
	assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(snapshot.Object, d1))
	assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, d2))

	assert.Equal(t, len(d1.Spec.Template.Spec.Volumes), len(d2.Spec.Template.Spec.Volumes),
		"ApplyMetricsTLS should not duplicate volumes")
	for i, c1 := range d1.Spec.Template.Spec.Containers {
		c2 := d2.Spec.Template.Spec.Containers[i]
		assert.Equal(t, len(c1.VolumeMounts), len(c2.VolumeMounts),
			"ApplyMetricsTLS should not duplicate volume mounts")
		assert.Equal(t, len(c1.Env), len(c2.Env),
			"ApplyMetricsTLS should not duplicate env vars")
	}
}

func TestApplyMetricsTLSDeploymentSkipsWrongName(t *testing.T) {
	ud := unstructuredDeployment(t) // name="registry"
	before := ud.DeepCopy()

	assert.NilError(t, ApplyMetricsTLS("Deployment", "other-name", "some-secret")(ud))

	// Unstructured content must be unchanged.
	assert.DeepEqual(t, before.Object, ud.Object)
}

// ---------------------------------------------------------------------------
// ApplyMetricsTLS — StatefulSet
// ---------------------------------------------------------------------------

func TestApplyMetricsTLSStatefulSet(t *testing.T) {
	const secretName = "results-watcher-metrics-tls"
	ud := unstructuredStatefulSet(t) // name="test-statefulset", one container

	assert.NilError(t, ApplyMetricsTLS("StatefulSet", "test-statefulset", secretName)(ud))

	sts := &appsv1.StatefulSet{}
	assert.NilError(t, scheme.Scheme.Convert(ud, sts, nil))

	spec := sts.Spec.Template.Spec

	assertVolumeSecret(t, spec.Volumes, metricsServingCertVolume, secretName)
	assertVolumeConfigMap(t, spec.Volumes, metricsClientCAVolume, MetricsClientCAConfigMap)

	for _, c := range spec.Containers {
		assertVolumeMount(t, c.VolumeMounts, metricsServingCertVolume, MetricsTLSMountPath)
		assertVolumeMount(t, c.VolumeMounts, metricsClientCAVolume, MetricsClientCAMountPath)
		assertEnv(t, c.Env, envMetricsTLSCert, fmt.Sprintf("%s/tls.crt", MetricsTLSMountPath))
		assertEnv(t, c.Env, envMetricsTLSKey, fmt.Sprintf("%s/tls.key", MetricsTLSMountPath))
		assertEnv(t, c.Env, envMetricsTLSClientCA, fmt.Sprintf("%s/%s", MetricsClientCAMountPath, MetricsClientCAKey))
		assertEnv(t, c.Env, envMetricsTLSClientAuth, "require")
	}
}

func TestApplyMetricsTLSStatefulSetIdempotent(t *testing.T) {
	const secretName = "results-watcher-metrics-tls"
	ud := unstructuredStatefulSet(t)

	assert.NilError(t, ApplyMetricsTLS("StatefulSet", "test-statefulset", secretName)(ud))
	snapshot := ud.DeepCopy()
	assert.NilError(t, ApplyMetricsTLS("StatefulSet", "test-statefulset", secretName)(ud))

	s1, s2 := &appsv1.StatefulSet{}, &appsv1.StatefulSet{}
	assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(snapshot.Object, s1))
	assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, s2))

	assert.Equal(t, len(s1.Spec.Template.Spec.Volumes), len(s2.Spec.Template.Spec.Volumes),
		"ApplyMetricsTLS should not duplicate volumes")
	for i, c1 := range s1.Spec.Template.Spec.Containers {
		c2 := s2.Spec.Template.Spec.Containers[i]
		assert.Equal(t, len(c1.VolumeMounts), len(c2.VolumeMounts),
			"ApplyMetricsTLS should not duplicate volume mounts")
		assert.Equal(t, len(c1.Env), len(c2.Env),
			"ApplyMetricsTLS should not duplicate env vars")
	}
}

// ---------------------------------------------------------------------------
// small assertion helpers
// ---------------------------------------------------------------------------

func assertVolumeSecret(t *testing.T, volumes []corev1.Volume, name, secretName string) {
	t.Helper()
	for _, v := range volumes {
		if v.Name == name {
			if v.Secret == nil {
				t.Errorf("volume %q: expected Secret source, got nil", name)
				return
			}
			assert.Equal(t, secretName, v.Secret.SecretName,
				"volume %q secretName", name)
			return
		}
	}
	t.Errorf("volume %q not found", name)
}

func assertVolumeConfigMap(t *testing.T, volumes []corev1.Volume, name, cmName string) {
	t.Helper()
	for _, v := range volumes {
		if v.Name == name {
			if v.ConfigMap == nil {
				t.Errorf("volume %q: expected ConfigMap source, got nil", name)
				return
			}
			assert.Equal(t, cmName, v.ConfigMap.LocalObjectReference.Name,
				"volume %q configMapName", name)
			return
		}
	}
	t.Errorf("volume %q not found", name)
}

func assertVolumeMount(t *testing.T, mounts []corev1.VolumeMount, name, mountPath string) {
	t.Helper()
	for _, m := range mounts {
		if m.Name == name {
			assert.Equal(t, mountPath, m.MountPath, "VolumeMount %q mountPath", name)
			assert.Assert(t, m.ReadOnly, "VolumeMount %q should be ReadOnly", name)
			return
		}
	}
	t.Errorf("VolumeMount %q not found", name)
}

func assertEnv(t *testing.T, envs []corev1.EnvVar, name, value string) {
	t.Helper()
	for _, e := range envs {
		if e.Name == name {
			assert.Equal(t, value, e.Value, "env %q value", name)
			return
		}
	}
	t.Errorf("env var %q not found", name)
}
