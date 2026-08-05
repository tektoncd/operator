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

package tektonscheduler

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

const schedulerCustomSet = "scheduler-network-policies"

func schedulerControllerDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(8443)

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "scheduler-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: schedulerControllerSelector,
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
	}
}

func schedulerWebhookDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	webhookPort := intstr.FromInt32(9443)
	metricsPort := intstr.FromInt32(8443)

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "scheduler-webhook"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: schedulerWebhookSelector,
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

var schedulerControllerSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{
		"app.kubernetes.io/name": "tekton-kueue",
		"control-plane":          "controller-manager",
	},
}

var schedulerWebhookSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{
		"app.kubernetes.io/name": "tekton-kueue-webhook",
		"control-plane":          "controller-manager",
	},
}

func schedulerDefaultDenyPolicies() []networkingv1.NetworkPolicy {
	return []networkingv1.NetworkPolicy{
		networkpolicy.DefaultDenyPolicy("scheduler-controller-default-deny", schedulerControllerSelector),
		networkpolicy.DefaultDenyPolicy("scheduler-webhook-default-deny", schedulerWebhookSelector),
	}
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, ts *v1alpha1.TektonScheduler) error {
	if ts.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, schedulerCustomSet)
	}
	defaults := schedulerDefaultDenyPolicies()
	defaults = append(defaults, schedulerControllerDefaultPolicies(r.platformParams)...)
	defaults = append(defaults, schedulerWebhookDefaultPolicies(r.platformParams)...)

	manifest, err := networkpolicy.Generate(
		ts.Spec.NetworkPolicy,
		ts.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, ts, schedulerCustomSet, &manifest, passthroughTransform, nil)
}

func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = passthroughTransform
