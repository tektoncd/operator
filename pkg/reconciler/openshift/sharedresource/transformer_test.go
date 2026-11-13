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

package sharedresource

import (
	"context"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

func TestFilterAndTransform_DaemonSetImages(t *testing.T) {
	t.Setenv(common.ImageRegistryOverride, "mirror.example.com/builds")
	t.Setenv(common.SharedResourceImagePrefix+"SHARED_RESOURCE_CSI_DRIVER", "ttl.sh/test-csi-driver:24h")
	t.Setenv(common.SharedResourceImagePrefix+"CSI_INIT", "ttl.sh/test-csi-init:24h")
	node := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      NodeDaemonSetName,
			Namespace: "openshift-builds",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "shared-resource-csi-driver", Image: "registry.redhat.io/openshift-builds/driver:old", SecurityContext: &corev1.SecurityContext{Privileged: ptr.Bool(true)}},
						{Name: "node-driver-registrar", Image: "registry.redhat.io/openshift4/registrar:old"},
					},
					InitContainers: []corev1.Container{
						{Name: "csi-init", Image: "registry.redhat.io/openshift-builds/init:old"},
						{Name: "csi-setup", Image: "registry.redhat.io/openshift-builds/setup:old"},
					},
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(node)
	assert.NilError(t, err)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{{Object: obj}}))
	assert.NilError(t, err)
	sr := &v1alpha1.SharedResource{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.SharedResourceResourceName},
		Spec: v1alpha1.SharedResourceSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"},
		},
	}
	ctx := context.Background()
	r := &Reconciler{extension: common.NoExtension(ctx)}

	out, err := FilterAndTransform(r)(ctx, &manifest, sr)
	assert.NilError(t, err)
	assert.Equal(t, len(out.Resources()), 1)
	got := &appsv1.DaemonSet{}
	assert.NilError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(out.Resources()[0].Object, got))
	assert.Equal(t, got.Namespace, "openshift-pipelines")
	assert.Equal(t, got.Labels[v1alpha1.LabelOperandName], OperandName)
	assert.Equal(t, got.Spec.Template.Spec.Containers[0].Image, "ttl.sh/test-csi-driver:24h")
	assert.Equal(t, got.Spec.Template.Spec.Containers[1].Image, "mirror.example.com/builds/openshift4/registrar:old")
	assert.Equal(t, got.Spec.Template.Spec.InitContainers[0].Image, "ttl.sh/test-csi-init:24h")
	assert.Equal(t, got.Spec.Template.Spec.InitContainers[1].Image, "mirror.example.com/builds/openshift-builds/setup:old")
	assert.Assert(t, *got.Spec.Template.Spec.Containers[0].SecurityContext.Privileged)
}
