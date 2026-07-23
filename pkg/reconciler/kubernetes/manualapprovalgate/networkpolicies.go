/*
Copyright 2024 The Tekton Authors

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

package manualapprovalgate

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func magDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(9090)
	webhookPort := intstr.FromInt32(8443)

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "mag-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-taskgroup-controller"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "mag-webhook"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "manual-approval-gate-webhook"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkpolicy.WebhookIngressRule("", webhookPort),
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
	}
}

func defaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy(
		"mag-default-deny",
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/part-of": "openshift-pipelines-manual-approval-gates",
			},
		},
	)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, mag *v1alpha1.ManualApprovalGate) error {
	if mag.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, "mag-network-policies")
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		magDefaultPolicies(r.platformParams)...,
	)
	manifest, err := networkpolicy.Generate(
		mag.Spec.NetworkPolicy,
		mag.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, mag, "mag-network-policies", &manifest, passthroughTransform, nil)
}

// passthroughTransform is a no-op FilterAndTransform used for pre-built manifests
// where namespace injection is already handled by Generate.
func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = passthroughTransform
