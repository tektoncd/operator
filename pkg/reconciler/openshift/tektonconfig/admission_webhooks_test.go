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

package tektonconfig

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestRemoveOperatorAdmissionWebhooks(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes namespace and proxy webhooks", func(t *testing.T) {
		kc := kubefake.NewSimpleClientset(
			&admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceAdmissionWebhookName},
			},
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: proxyAdmissionWebhookName},
			},
		)

		err := removeOperatorAdmissionWebhooks(ctx, kc)
		assert.NilError(t, err)

		_, err = kc.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, namespaceAdmissionWebhookName, metav1.GetOptions{})
		assert.ErrorContains(t, err, "not found")

		_, err = kc.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, proxyAdmissionWebhookName, metav1.GetOptions{})
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("succeeds when webhooks are already gone", func(t *testing.T) {
		kc := kubefake.NewSimpleClientset()
		err := removeOperatorAdmissionWebhooks(ctx, kc)
		assert.NilError(t, err)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		kc := kubefake.NewSimpleClientset(
			&admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceAdmissionWebhookName},
			},
		)
		kc.PrependReactor("delete", "validatingwebhookconfigurations", func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("delete failed")
		})

		err := removeOperatorAdmissionWebhooks(ctx, kc)
		assert.ErrorContains(t, err, "failed to delete validating webhook")
	})
}
