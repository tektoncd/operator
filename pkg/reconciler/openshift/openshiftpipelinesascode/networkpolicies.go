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

package openshiftpipelinesascode

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

func pacDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(9090)
	controllerAPIPort := intstr.FromInt32(8082)
	webhookPort := intstr.FromInt32(8443)

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pac-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "pipelines-as-code-controller"},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkpolicy.PrometheusIngressRule(params, metricsPort),
					// Git provider webhooks arrive via Route/Ingress — allow from any source.
					networkpolicy.WebhookIngressRule("", controllerAPIPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
					networkpolicy.InternetEgressRule(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pac-watcher"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "pipelines-as-code-watcher"},
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
			ObjectMeta: metav1.ObjectMeta{Name: "pac-webhook"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "webhook",
						"app.kubernetes.io/part-of":   "pipelines-as-code",
					},
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

func pacDefaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy(
		"pac-default-deny",
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/part-of": "pipelines-as-code",
			},
		},
	)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, pac *v1alpha1.OpenShiftPipelinesAsCode) error {
	if pac.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, "pac-network-policies")
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{pacDefaultDenyPolicy()},
		pacDefaultPolicies(r.platformParams)...,
	)
	manifest, err := networkpolicy.Generate(
		pac.Spec.NetworkPolicy,
		pac.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, pac, "pac-network-policies", &manifest, passthroughTransform, nil)
}

// passthroughTransform is a no-op FilterAndTransform used for pre-built manifests
// where namespace injection is already handled by Generate.
func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = passthroughTransform
