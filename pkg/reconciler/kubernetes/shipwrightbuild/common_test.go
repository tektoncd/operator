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

package shipwrightbuild

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateWebhookSecret(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset()
	namespace := "openshift-builds"
	ownerRef := metav1.OwnerReference{
		APIVersion: "operator.tekton.dev/v1alpha1",
		Kind:       "ShipwrightBuild",
		Name:       "shipwright-build",
	}

	// The secret does not exist yet, so it is created.
	err := CreateWebhookSecret(ctx, client, namespace, ownerRef)
	util.AssertEqual(t, err, nil)

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, WebhookSecretName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, secret.Name, WebhookSecretName)

	// The generated secret carries the certificate keypair.
	for _, key := range []string{common.ServerKey, common.ServerCert, common.CACert} {
		if _, ok := secret.Data[key]; !ok {
			t.Errorf("expected secret to contain data key %q", key)
		}
	}

	// The owner reference is propagated so the secret is garbage collected with the CR.
	util.AssertEqual(t, len(secret.OwnerReferences), 1)
	util.AssertEqual(t, secret.OwnerReferences[0].Name, ownerRef.Name)
}

func TestCreateWebhookSecretAlreadyExists(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	namespace := "openshift-builds"
	ownerRef := metav1.OwnerReference{Name: "shipwright-build"}

	// First call creates the secret.
	err := CreateWebhookSecret(ctx, client, namespace, ownerRef)
	util.AssertEqual(t, err, nil)

	first, err := client.CoreV1().Secrets(namespace).Get(ctx, WebhookSecretName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)

	// Second call is a no-op because the secret already exists; it must not be regenerated.
	err = CreateWebhookSecret(ctx, client, namespace, ownerRef)
	util.AssertEqual(t, err, nil)

	second, err := client.CoreV1().Secrets(namespace).Get(ctx, WebhookSecretName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertDeepEqual(t, second.Data, first.Data)
}
