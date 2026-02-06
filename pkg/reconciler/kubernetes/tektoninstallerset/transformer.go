/*
Copyright 2021 The Tekton Authors

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

package tektoninstallerset

import (
	"context"

	mf "github.com/manifestival/manifestival"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

func injectOwner(owner []v1.OwnerReference) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if kind == "CustomResourceDefinition" ||
			kind == "ValidatingWebhookConfiguration" ||
			kind == "MutatingWebhookConfiguration" ||
			kind == "Namespace" {
			return nil
		}
		u.SetOwnerReferences(owner)
		return nil
	}
}

func injectOwnerForCRDsAndNamespace(owner []v1.OwnerReference) mf.Transformer {
	if len(owner) == 0 {
		return func(u *unstructured.Unstructured) error { return nil }
	}
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if kind != "CustomResourceDefinition" &&
			kind != "Namespace" {
			return nil
		}
		u.SetOwnerReferences(owner)
		return nil
	}
}

// injectNamespaceOwnerForOperatorWebhooks sets namespace as owner for operator webhooks
// to ensure they are garbage collected when the namespace is deleted (SRVKP-8901).
// Only targets proxy.operator.tekton.dev and namespace.operator.tekton.dev webhooks.
func injectNamespaceOwnerForOperatorWebhooks(kubeClient kubernetes.Interface, targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		name := u.GetName()

		// Only apply to operator webhooks, not pipeline/triggers/PAC webhooks
		if (kind == "MutatingWebhookConfiguration" && name == "proxy.operator.tekton.dev") ||
			(kind == "ValidatingWebhookConfiguration" && name == "namespace.operator.tekton.dev") {

			// Get target namespace (where webhooks are deployed, not where operator runs)
			ns, err := kubeClient.CoreV1().Namespaces().Get(context.TODO(), targetNamespace, v1.GetOptions{})
			if err != nil {
				// Log but don't fail - webhook will work without ownerRef
				return nil
			}

			// Set namespace as owner (without BlockOwnerDeletion/Controller to avoid RBAC issues)
			u.SetOwnerReferences([]v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Namespace",
					Name:       ns.Name,
					UID:        ns.UID,
				},
			})
		}
		return nil
	}
}
