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
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NetworkPolicySet is the independently versioned CustomSet that
// carries the Shipwright Build NetworkPolicies.
const NetworkPolicySet = "shipwrightbuild-network-policies"

// defaultPolicies returns the default NetworkPolicies for the two
// Shipwright Build workloads shipped in the release manifest:
//
//   - the controller Deployment (pod label name=shipwright-build), which serves
//     metrics on 8383, and
//   - the webhook Deployment (pod label name=shp-build-webhook), which serves the
//     admission webhook on 8443.
func defaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(8383)
	webhookPort := intstr.FromInt32(8443)

	return []networkingv1.NetworkPolicy{
		{
			Name: shipwrightbuild.ControllerDeploymentName,
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: shipwrightbuild.ControllerLabels,
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
			Name: shipwrightbuild.WebhookDeploymentName,
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: shipwrightbuild.WebhookLabels,
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					// Admission webhook requests arrive from the kube-apiserver.
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

// defaultDenyPolicy denies all ingress/egress for pods carrying the
// Shipwright Build operand label, so the per-workload policies above are additive.
func defaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy(
		shipwrightbuild.OperandName+"-default-deny",
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1alpha1.LabelOperandName: shipwrightbuild.OperandName,
			},
		},
	)
}
