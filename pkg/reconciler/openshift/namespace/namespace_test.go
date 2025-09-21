/*
Copyright 2023 The Tekton Authors

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

package namespace

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	operatorinformers "github.com/tektoncd/operator/pkg/client/informers/externalversions"
	"gotest.tools/v3/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestReconciler_Admit_BasicOperations(t *testing.T) {
	tests := []struct {
		name        string
		operation   admissionv1.Operation
		wantAllowed bool
	}{
		{
			name:        "create operation should be processed",
			operation:   admissionv1.Create,
			wantAllowed: true,
		},
		{
			name:        "update operation should be processed",
			operation:   admissionv1.Update,
			wantAllowed: true,
		},
		{
			name:        "delete operation should be allowed (unhandled)",
			operation:   admissionv1.Delete,
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal reconciler
			r := &reconciler{}

			// Create simple namespace
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			}

			namespaceBytes, err := json.Marshal(namespace)
			assert.NilError(t, err)

			req := &admissionv1.AdmissionRequest{
				Kind: metav1.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "Namespace",
				},
				Object: runtime.RawExtension{
					Raw: namespaceBytes,
				},
				Operation: tt.operation,
			}

			ctx := logging.WithLogger(context.Background(), logtesting.TestLogger(t))
			response := r.Admit(ctx, req)

			assert.Assert(t, response != nil, "response should not be nil")
			assert.Equal(t, tt.wantAllowed, response.Allowed)
		})
	}
}

func TestReconciler_admissionAllowed_NoSCCAnnotation(t *testing.T) {
	// Create namespace without SCC annotation
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			// No annotations
		},
	}

	// Create minimal TektonConfig
	tektonConfig := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
	}

	// Setup fake client and informer
	operatorClient := operatorfake.NewSimpleClientset(tektonConfig)
	operatorInformerFactory := operatorinformers.NewSharedInformerFactory(operatorClient, 0)
	tektonConfigInformer := operatorInformerFactory.Operator().V1alpha1().TektonConfigs()

	err := tektonConfigInformer.Informer().GetStore().Add(tektonConfig)
	assert.NilError(t, err)

	r := &reconciler{
		tektonConfigLister: tektonConfigInformer.Lister(),
	}

	// Create admission request
	namespaceBytes, err := json.Marshal(namespace)
	assert.NilError(t, err)

	req := &admissionv1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Namespace",
		},
		Object: runtime.RawExtension{
			Raw: namespaceBytes,
		},
		Operation: admissionv1.Create,
	}

	allowed, status, err := r.admissionAllowed(context.Background(), req)

	// Should be allowed since no SCC annotation
	assert.NilError(t, err)
	assert.Equal(t, true, allowed)
	assert.Assert(t, status == nil, "status should be nil")
}

func TestReconciler_admissionAllowed_InvalidKind(t *testing.T) {
	r := &reconciler{}

	req := &admissionv1.AdmissionRequest{
		Kind: metav1.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		},
		Object: runtime.RawExtension{
			Raw: []byte(`{}`),
		},
		Operation: admissionv1.Create,
	}

	allowed, status, err := r.admissionAllowed(context.Background(), req)

	// Should return error for invalid kind
	assert.Assert(t, err != nil, "expected error for invalid kind")
	assert.Equal(t, false, allowed)
	assert.Assert(t, status == nil, "status should be nil for error")
}
