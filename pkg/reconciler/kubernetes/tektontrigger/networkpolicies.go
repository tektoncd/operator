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

package tektontrigger

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func triggersDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(9000)
	webhookPort := intstr.FromInt32(8443)
	interceptorPort := intstr.FromInt32(8443)
	tcp := corev1.ProtocolTCP

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "triggers-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-triggers-controller"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(params),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "triggers-webhook"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-triggers-webhook"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					// cidr="" → allow from any source; operator defaults to permissive.
					// Users can restrict to a specific control-plane CIDR via spec.networkPolicy.policies.
					networkpolicy.WebhookIngressRule("", webhookPort),
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(params),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "triggers-core-interceptors"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-triggers-core-interceptors"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						// EventListeners live in user-controlled namespaces — allow ingress from all namespaces.
						From: []networkingv1.NetworkPolicyPeer{
							{NamespaceSelector: &metav1.LabelSelector{}},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &tcp, Port: &interceptorPort},
						},
					},
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(params),
					// Core interceptors may call external APIs (e.g. GitHub) to fetch files,
					// verify ownership, or perform other outbound operations.
					networkpolicy.InternetEgressRule(),
				},
			},
		},
	}
}

// defaultDenyPolicy returns the default-deny NetworkPolicy for the Tekton namespace.
//
// Naming: the policy is intentionally named "tekton-default-deny" (not
// "tekton-triggers-default-deny"). The long-term design is for a single
// namespace-wide deny to be owned by one component (TektonPipeline), so all
// components converge on the same name. Do NOT rename this to a component-specific
// name — doing so would leave orphaned per-component deny policies once the
// namespace-wide policy takes over, and would break the migration path.
//
// Temporary placement: this lives here because TektonTrigger is the first component
// to implement NetworkPolicy support. TektonConfig cannot own it (no InstallerSetClient),
// so the intended long-term owner is TektonPipeline, which is the foundational component
// that all others depend on and already has an InstallerSetClient.
//
// Migration path: once all components (Pipeline, Chains, Results, Dashboard…) implement
// NetworkPolicy support, this function should move to the TektonPipeline reconciler,
// the pod selector should be widened to an empty metav1.LabelSelector{} (namespace-wide
// deny), and this copy removed.
func defaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy(
		"tekton-default-deny",
		metav1.LabelSelector{
			MatchLabels: map[string]string{"app.kubernetes.io/part-of": "tekton-triggers"},
		},
	)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, tt *v1alpha1.TektonTrigger) error {
	if tt.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, "triggers-network-policies")
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		triggersDefaultPolicies(r.platformParams)...,
	)
	manifest, err := networkpolicy.Generate(
		tt.Spec.NetworkPolicy,
		tt.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, tt, "triggers-network-policies", &manifest, passthroughTransform, nil)
}

// passthroughTransform is a no-op FilterAndTransform used for pre-built manifests
// where namespace injection is already handled by Generate.
func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

// Ensure passthroughTransform satisfies the FilterAndTransform type.
var _ client.FilterAndTransform = passthroughTransform
