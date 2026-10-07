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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

type kubernetesExtension struct {
	kubeClientset kubernetes.Interface
}

// NewKubernetesExtension builds the extension for Shipwright Build
func NewKubernetesExtension(ctx context.Context) common.Extension {
	return kubernetesExtension{
		kubeClientset: kubeclient.Get(ctx),
	}
}

func (e kubernetesExtension) Transformers(tc v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}

func (e kubernetesExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	n := tc.GetSpec().GetTargetNamespace()
	or := *metav1.NewControllerRef(tc, tc.GroupVersionKind())
	if err := CreateWebhookSecret(ctx, e.kubeClientset, n, or); err != nil {
		return err
	}
	return nil
}

func (e kubernetesExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	return nil
}

func (e kubernetesExtension) Finalize(ctx context.Context, tc v1alpha1.TektonComponent) error {
	return nil
}

func (e kubernetesExtension) GetPlatformData() string {
	return ""
}
