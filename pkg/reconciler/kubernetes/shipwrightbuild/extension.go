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
	"fmt"

	certmanagerclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
)

// kubernetesExtension holds the Kubernetes-platform hooks for Shipwright Build; it
// needs the kube client to provision the webhook's TLS secret before install.
type kubernetesExtension struct {
	kubernetesClient  kubernetes.Interface
	certManagerClient certmanagerclient.Interface
}

// NewKubernetesExtension builds the extension for Shipwright Build
func NewKubernetesExtension(ctx context.Context) common.Extension {
	certManagerClient := certmanagerclient.NewForConfigOrDie(injection.GetConfig(ctx))

	return kubernetesExtension{
		kubernetesClient:  kubeclient.Get(ctx),
		certManagerClient: certManagerClient,
	}
}

func (ke kubernetesExtension) Transformers(tc v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		InjectCertManagerCABundle(common.KindCustomResourceDefinition, WebhookSecretName, tc.GetSpec().GetTargetNamespace(), nil),
	}
}

// PreReconcile ensures the webhook TLS secret exists before the manifests are
// applied, since the webhook deployment mounts it at startup.
func (ke kubernetesExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	sb := tc.(*v1alpha1.ShipwrightBuild)

	// Cert-Manager should also be pre-installed
	if err := IsCertManagerInstalled(ke.kubernetesClient.Discovery()); err != nil {
		sb.Status.MarkDependencyMissing(fmt.Sprintf("API %s does not exists, please install.", CertManagerGroupVersion))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	namespace := tc.GetSpec().GetTargetNamespace()
	ownerReference := *metav1.NewControllerRef(tc, tc.GroupVersionKind())
	if err := EnsureWebhookCertificate(ctx, ke.certManagerClient, namespace, ownerReference); err != nil {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	return nil
}

func (ke kubernetesExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	return nil
}

func (ke kubernetesExtension) Finalize(ctx context.Context, tc v1alpha1.TektonComponent) error {
	return nil
}

func (ke kubernetesExtension) GetPlatformData() string {
	return ""
}
