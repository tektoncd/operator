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

package tektonpipeline

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

// proxyWebhookPodSelector matches the proxy-webhook Deployment/Service selector
// (name: tekton-operator) shipped in cmd/{kubernetes,openshift}/operator/kodata/webhook/webhook.yaml.
// This label is the same value used by the main operator Deployment in the operator's own
// namespace (see tektoncd/operator#3227) — harmless here because the proxy-webhook runs in the
// operand namespace (e.g. tekton-pipelines / openshift-pipelines), never the operator namespace.
var proxyWebhookPodSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{"name": "tekton-operator"},
}

// proxyWebhookDefaultPolicies returns the default NetworkPolicies for the proxy-webhook
// workload specifically. As more TektonPipeline-managed workloads gain NetworkPolicy
// support, they should get their own <workload>DefaultPolicies function rather than
// growing this one, so reconcileNetworkPolicies can just concatenate each workload's
// defaults (see triggersDefaultPolicies in tektontrigger for the alternative — a single
// function covering multiple workloads — which this intentionally avoids).
func proxyWebhookDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	webhookPort := intstr.FromInt32(8443)

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "proxy-webhook"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: proxyWebhookPodSelector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					// cidr="" → permissive; restrict via spec.networkPolicy.policies if needed.
					networkpolicy.WebhookIngressRule("", webhookPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
	}
}

// proxyWebhookDefaultDenyPolicy returns the scoped default-deny for the proxy-webhook pod
// only. It is not a component-wide default-deny — each TektonPipeline-managed workload
// that gains NetworkPolicy support should define its own <workload>DefaultDenyPolicy.
func proxyWebhookDefaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy("tekton-proxy-webhook-default-deny", proxyWebhookPodSelector)
}

func pipelineDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(9090)
	webhookPort := intstr.FromInt32(8443)
	resolverPort := intstr.FromInt32(8080)
	tcp := corev1.ProtocolTCP

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-pipelines-controller"},
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
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline-webhook"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-pipelines-webhook"},
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
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline-events-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-events-controller"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
					networkpolicy.InternetEgressRule(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pipeline-resolvers"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-pipelines-resolvers"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "tekton-pipelines-controller"},
							}},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &tcp, Port: &resolverPort},
						},
					},
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
					networkpolicy.InternetEgressRule(),
					networkpolicy.SSHEgressRule(),
				},
			},
		},
	}
}

func pipelineDefaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy(
		"pipeline-default-deny",
		metav1.LabelSelector{
			MatchLabels: map[string]string{"app.kubernetes.io/part-of": "tekton-pipelines"},
		},
	)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, tp *v1alpha1.TektonPipeline) error {
	if tp.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, "pipeline-network-policies")
	}
	defaults := []networkingv1.NetworkPolicy{
		proxyWebhookDefaultDenyPolicy(),
		pipelineDefaultDenyPolicy(),
	}
	defaults = append(defaults, proxyWebhookDefaultPolicies(r.platformParams)...)
	defaults = append(defaults, pipelineDefaultPolicies(r.platformParams)...)

	manifest, err := networkpolicy.Generate(
		tp.Spec.NetworkPolicy,
		tp.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, tp, "pipeline-network-policies", &manifest, passthroughTransform, nil)
}

// passthroughTransform is a no-op FilterAndTransform used for pre-built manifests
// where namespace injection is already handled by Generate.
func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

// Ensure passthroughTransform satisfies the FilterAndTransform type.
var _ client.FilterAndTransform = passthroughTransform
