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

package tektoninstallerset

import (
	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// crdUpdateClient retains Manifestival's merge semantics, but avoids storing a
// full CRD schema in an annotation when that exceeds Kubernetes' annotation
// limit. InstallerSet's last-applied hash still tracks the desired manifest.
// This matters when adopting CRDs created by another operator: creation does
// not use Manifestival.Apply, whereas updating an existing CRD does.
type crdUpdateClient struct {
	mf.Client
}

func (c crdUpdateClient) Update(resource *unstructured.Unstructured, options ...mf.ApplyOption) error {
	annotations := resource.GetAnnotations()
	if resource.GetKind() == "CustomResourceDefinition" && validation.ValidateAnnotationsSize(annotations) != nil {
		delete(annotations, corev1.LastAppliedConfigAnnotation)
		resource.SetAnnotations(annotations)
	}
	return c.Client.Update(resource, options...)
}
