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
	mf "github.com/manifestival/manifestival"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
