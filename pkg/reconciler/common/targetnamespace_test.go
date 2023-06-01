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
	"context"
	"testing"
	"time"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReconcileTargetNamespace(t *testing.T) {
	namespaceTektonPipelines := "tekton-pipelines"

	tests := []struct {
		name             string
		component        v1alpha1.TektonComponent
		additionalLabels map[string]string
		ownerReferences  []metav1.OwnerReference
		preFunc          func(t *testing.T, fakeClientset *fake.Clientset)                              // preFunc used to update namespace details before reconcile
		err              error                                                                          // error if any from ReconcileTargetNamespace function
		postFunc         func(t *testing.T, fakeClientset *fake.Clientset, namespace *corev1.Namespace) // postFunc used to verify additional fields
	}{
		{
			name: "verify-tekton-config",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			err: nil,
		},
		{
			name: "verify-tekton-hub",
			component: &v1alpha1.TektonHub{
				ObjectMeta: metav1.ObjectMeta{Name: "hub"},
				Spec:       v1alpha1.TektonHubSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			err: nil,
		},
		{
			name: "verify-custom-target-namespace",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "hello123"}},
			},
			err: nil,
		},
		{
			name: "verify-with-additional-labels",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			additionalLabels: map[string]string{
				"openshift.io/cluster-monitoring": "true",
			},
			err: nil,
		},
		{
			name: "verify-with-expected-labels",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			preFunc: func(t *testing.T, fakeClientset *fake.Clientset) {
				// create a namespace with "operator.tekton.dev/targetNamespace" label
				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: namespaceTektonPipelines,
					Labels: map[string]string{
						// one of expected label
						labelKeyTargetNamespace: "true",
					},
				}}
				_, err := fakeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
				assert.NilError(t, err)
			},
			err: nil,
		},
		{
			name: "verify-existing-non-target-namespace-deleted",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "hello123"}},
			},
			preFunc: func(t *testing.T, fakeClientset *fake.Clientset) {
				// create a namespace with different name and with "operator.tekton.dev/targetNamespace" label
				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: "custom-ns",
					Labels: map[string]string{
						labelKeyTargetNamespace: "true",
					},
				}}
				_, err := fakeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
				assert.NilError(t, err)
			},
			postFunc: func(t *testing.T, fakeClientset *fake.Clientset, namespace *corev1.Namespace) {
				// verify "custom-ns" is removed
				_, err := fakeClientset.CoreV1().Namespaces().Get(context.TODO(), "custom-ns", metav1.GetOptions{})
				assert.Equal(t, true, errors.IsNotFound(err), "'custom-ns' namespace should be deleted, but still found")
			},
			err: nil,
		},
		{
			name: "verify-namespace-in-deletion-state",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			preFunc: func(t *testing.T, fakeClientset *fake.Clientset) {
				// create a namespace with deletionTimestamp, it means the namespace is in deletion state
				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name:              namespaceTektonPipelines,
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				}}
				_, err := fakeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
				assert.NilError(t, err)
			},
			// ReconcileTargetNamespace requeue event for the namespace is in deletion state
			err: v1alpha1.REQUEUE_EVENT_AFTER,
		},
		{
			name: "verify-non-target-namespace-in-deletion-state",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			preFunc: func(t *testing.T, fakeClientset *fake.Clientset) {
				// create a namespace with deletionTimestamp, it means the namespace is in deletion state
				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: "custom-ns",
					Labels: map[string]string{
						labelKeyTargetNamespace: "true",
					},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				}}
				_, err := fakeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
				assert.NilError(t, err)
			},
			// ReconcileTargetNamespace requeue event for the namespace is in deletion state
			err: v1alpha1.REQUEUE_EVENT_AFTER,
		},
		{
			name: "verify-existing-namespace",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			preFunc: func(t *testing.T, fakeClientset *fake.Clientset) {
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceTektonPipelines,
						// additional labels
						Labels: map[string]string{
							"custom": "123",
							"test":   "hello",
						},
						Annotations: map[string]string{
							"custom-annotations": "tekton",
							"key123":             "value123",
						},
					},
				}
				_, err := fakeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
				assert.NilError(t, err)
			},
			postFunc: func(t *testing.T, fakeClientset *fake.Clientset, namespace *corev1.Namespace) {
				// verify custom labels
				expectedLabels := map[string]string{
					"custom": "123",
					"test":   "hello",
				}
				for expectedLabelKey, expectedLabelValue := range expectedLabels {
					labelFound := false
					for actualLabelKey, actualLabelValue := range namespace.GetLabels() {
						if expectedLabelKey == actualLabelKey && expectedLabelValue == actualLabelValue {
							labelFound = true
							break
						}
					}
					assert.Equal(t, true, labelFound, "expected labelKey or labelValue not found:[%s=%s]", expectedLabelKey, expectedLabelValue)
				}
				// verify existing annotations
				expectedAnnotations := map[string]string{
					"custom-annotations": "tekton",
					"key123":             "value123",
				}
				for expectedAnnotationKey, expectedAnnotationValue := range expectedAnnotations {
					annotationFound := false
					for actualAnnotationKey, actualAnnotationValue := range namespace.GetAnnotations() {
						if expectedAnnotationKey == actualAnnotationKey && expectedAnnotationValue == actualAnnotationValue {
							annotationFound = true
							break
						}
					}
					assert.Equal(t, true, annotationFound, "expected annotationKey or annotationValue not found:[%s=%s]", expectedAnnotationKey, expectedAnnotationValue)
				}
			},
			err: nil,
		},
		{
			name: "verify-existing-namespace-with-owner-reference",
			component: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec:       v1alpha1.TektonConfigSpec{CommonSpec: v1alpha1.CommonSpec{TargetNamespace: namespaceTektonPipelines}},
			},
			additionalLabels: map[string]string{},
			ownerReferences:  []metav1.OwnerReference{*metav1.NewControllerRef(&corev1.Namespace{}, (&corev1.Namespace{}).GroupVersionKind())},
			preFunc: func(t *testing.T, fakeClientset *fake.Clientset) {
				ns := &corev1.Namespace{}
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namespaceTektonPipelines,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ns, ns.GroupVersionKind())},
					},
				}
				_, err := fakeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
				assert.NilError(t, err)
			},
			err: nil,
		},
	}

	for _, test := range tests {
		// execute tests
		t.Run(test.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()
			targetNamespace := test.component.GetSpec().GetTargetNamespace()
			expectedLabels := map[string]string{labelKeyTargetNamespace: "true"}
			// include additional labels, if any
			if len(test.additionalLabels) > 0 {
				for k, v := range test.additionalLabels {
					expectedLabels[k] = v
				}
			}
			// create expected owner reference for that namespace and compute hash
			expectedOwnerRef := []metav1.OwnerReference{*metav1.NewControllerRef(test.component, test.component.GroupVersionKind())}
			if len(test.ownerReferences) > 0 {
				expectedOwnerRef = test.ownerReferences
			}
			expectedHash, err := hash.Compute(expectedOwnerRef)
			assert.NilError(t, err)

			// execute pre function
			if test.preFunc != nil {
				test.preFunc(t, fakeClientset)
			}

			// call reconciler
			err = ReconcileTargetNamespace(context.Background(), test.additionalLabels, test.component, fakeClientset)
			assert.Equal(t, err, test.err)

			if test.err == nil {
				namespace, err := fakeClientset.CoreV1().Namespaces().Get(context.Background(), targetNamespace, metav1.GetOptions{})
				assert.NilError(t, err)
				assert.Equal(t, namespace.ObjectMeta.Name, targetNamespace)

				// verify labels
				for expectedLabelKey, expectedLabelValue := range expectedLabels {
					labelFound := false
					for actualLabelKey, actualLabelValue := range namespace.GetLabels() {
						if expectedLabelKey == actualLabelKey && expectedLabelValue == actualLabelValue {
							labelFound = true
							break
						}
					}
					assert.Equal(t, true, labelFound, "expected labelKey or labelValue not found:[%s=%s]", expectedLabelKey, expectedLabelValue)
				}

				// verify owner reference
				assert.Equal(t, 1, len(namespace.GetOwnerReferences()))
				actualHash, err := hash.Compute(namespace.GetOwnerReferences())
				assert.NilError(t, err)
				assert.Equal(t, expectedHash, actualHash)

				// execute post function
				if test.postFunc != nil {
					test.postFunc(t, fakeClientset, namespace)
				}
			}
		})
	}
}
