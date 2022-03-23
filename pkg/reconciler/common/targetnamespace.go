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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateTargetNamespace(ctx context.Context, labels map[string]string, obj v1alpha1.TektonComponent, kubeClientSet kubernetes.Interface) error {
	ownerRef := *metav1.NewControllerRef(obj, obj.GroupVersionKind())
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: obj.GetSpec().GetTargetNamespace(),
			Labels: map[string]string{
				"operator.tekton.dev/targetNamespace": "true",
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}

	if len(labels) > 0 {
		for key, value := range labels {
			namespace.Labels[key] = value
		}
	}

	if _, err := kubeClientSet.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}
