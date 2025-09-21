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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/markbates/inflect"
	"github.com/tektoncd/operator/pkg/client/listers/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	"go.uber.org/zap"

	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key  types.NamespacedName
	path string

	withContext func(context.Context) context.Context

	client             kubernetes.Interface
	vwhlister          admissionlisters.ValidatingWebhookConfigurationLister
	secretlister       corelisters.SecretLister
	tektonConfigLister v1alpha1.TektonConfigLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !ac.IsLeaderFor(ac.key) {
		logger.Debugf("Skipping key %q, not the leader.", ac.key)
		return nil
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}
	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileValidatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	logger := logging.FromContext(ctx)

	// Need to handle both, create and update operations for a namespace
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
	default:
		logger.Info("Unhandled webhook operation, letting it through ", request.Operation)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	isAllowed, status, err := ac.admissionAllowed(ctx, request)
	if err != nil {
		return webhook.MakeErrorStatus("admission failed for namespace %v", err)
	}

	// Something in status means that admission isn't allowed
	if status != nil {
		return &admissionv1.AdmissionResponse{
			// isAllowed should be false here always
			Allowed: isAllowed,
			Result:  status,
		}
	}

	return &admissionv1.AdmissionResponse{
		// At this point, isAllowed should always be true
		Allowed: isAllowed,
	}
}

func (ac *reconciler) admissionAllowed(ctx context.Context, req *admissionv1.AdmissionRequest) (bool, *metav1.Status, error) {
	kind := req.Kind
	namespaceRawBytes := req.Object.Raw

	// Why, oh why are these different types...
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	logger := logging.FromContext(ctx)
	if gvk.Group != "" || gvk.Version != "v1" || gvk.Kind != "Namespace" {
		logger.Error("Unhandled kind: ", gvk)
		return false, nil, fmt.Errorf("unhandled kind: %v", gvk)
	}

	// nil values denote absence of `old` (create) or `new` (delete) objects.
	var namespaceObject corev1.Namespace

	if len(namespaceRawBytes) != 0 {
		newDecoder := json.NewDecoder(bytes.NewBuffer(namespaceRawBytes))
		if ac.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&namespaceObject); err != nil {
			return false, nil, fmt.Errorf("cannot decode incoming new object: %w", err)
		}
	}

	nsSCC := namespaceObject.Annotations[openshift.NamespaceSCCAnnotation]
	// If no annotation in namespace, then nothing to do here
	if nsSCC == "" {
		return true, nil, nil
	}

	logger.Infof("Trying to admit namespace: %s with SCC: %s", namespaceObject.Name, nsSCC)

	securityClient := common.GetSecurityClient(ctx)

	// verify SCC exists on the cluster
	_, err := securityClient.SecurityV1().SecurityContextConstraints().Get(ctx, nsSCC, metav1.GetOptions{})
	if err != nil {
		return false, nil, err
	}

	tc, err := ac.tektonConfigLister.Get("config")
	if err != nil {
		return false, nil, err
	}

	// Check if the SCC requested in namespace is in line with the maxAllowed SCC in TektonConfig
	maxAllowedSCC := tc.Spec.Platforms.OpenShift.SCC.MaxAllowed

	// If no maxAllowed is set, no problem
	if maxAllowedSCC == "" {
		logger.Infof("Namespace %s validation: no maxAllowed SCC set in TektonConfig", namespaceObject.Name)
		return true, nil, nil
	}

	prioritizedSCCList, err := common.GetSCCRestrictiveList(ctx, securityClient)
	if err != nil {
		return false, nil, err
	}

	isPriority, err := common.SCCAMoreRestrictiveThanB(prioritizedSCCList, nsSCC, maxAllowedSCC)
	if err != nil {
		return false, nil, err
	}
	logger.Infof("Is maxAllowed SCC: %s less restrictive than namespace SCC: %s? %t", maxAllowedSCC, nsSCC, isPriority)
	if !isPriority {
		prioErr := fmt.Sprintf("namespace: %s has requested SCC: %s, but it is less restrictive than 'maxAllowed' SCC: %s", namespaceObject.Name, nsSCC, maxAllowedSCC)
		return false, &metav1.Status{
			Status:  "Failure",
			Message: prioErr,
		}, nil
	}

	return true, nil, nil
}

func (ac *reconciler) reconcileValidatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	pluralNS := strings.ToLower(inflect.Pluralize("Namespace"))
	rules := []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{pluralNS, pluralNS + "/status"},
			},
		},
	}

	configuredWebhook, err := ac.vwhlister.Get(ac.key.Name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %w", err)
	}

	webhook := configuredWebhook.DeepCopy()

	// Clear out any previous (bad) OwnerReferences.
	// See: https://github.com/knative/serving/issues/5845
	webhook.OwnerReferences = nil

	for i, wh := range webhook.Webhooks {
		if wh.Name != webhook.Name {
			continue
		}
		webhook.Webhooks[i].Rules = rules
		webhook.Webhooks[i].NamespaceSelector = &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					// "control-plane" is added to support Azure's AKS, otherwise the controllers fight.
					// See knative/pkg#1590 for details.
					Key:      "control-plane",
					Operator: metav1.LabelSelectorOpDoesNotExist,
				},
			},
		}
		// Exclude system namespaces
		webhook.Webhooks[i].MatchConditions = []admissionregistrationv1.MatchCondition{
			{
				Name:       "exclude-system-namespaces",
				Expression: "!(object.metadata.name.startsWith('kube-') || object.metadata.name.startsWith('openshift-'))",
			},
		}

		webhook.Webhooks[i].ClientConfig.CABundle = caCert
		if webhook.Webhooks[i].ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		vwhclient := ac.client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
		if _, err := vwhclient.Update(ctx, webhook, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}
