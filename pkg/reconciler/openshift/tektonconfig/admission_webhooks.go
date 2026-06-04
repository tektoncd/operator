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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	namespaceAdmissionWebhookName = "namespace.operator.tekton.dev"
	proxyAdmissionWebhookName     = "proxy.operator.tekton.dev"
)

// removeOperatorAdmissionWebhooks deletes operator admission webhooks before namespace
// label cleanup during TektonConfig finalization. This prevents finalize from failing when
// the openshift-pipelines operand namespace (and proxy-webhook service) is already gone
// but the webhook configurations still exist (SRVKP-12010).
func removeOperatorAdmissionWebhooks(ctx context.Context, kubeClient kubernetes.Interface) error {
	if err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(
		ctx, namespaceAdmissionWebhookName, metav1.DeleteOptions{},
	); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete validating webhook %s: %w", namespaceAdmissionWebhookName, err)
	}

	if err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(
		ctx, proxyAdmissionWebhookName, metav1.DeleteOptions{},
	); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete mutating webhook %s: %w", proxyAdmissionWebhookName, err)
	}

	return nil
}
