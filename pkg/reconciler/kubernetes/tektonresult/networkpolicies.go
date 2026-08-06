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

package tektonresult

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

const (
	defaultAPIPort        int32 = 8080
	defaultPrometheusPort int32 = 9090
	defaultDBPort         int32 = 5432
)

func resultsDefaultPolicies(params networkpolicy.PlatformParams, props v1alpha1.ResultsAPIProperties) []networkingv1.NetworkPolicy {
	apiPort := intstr.FromInt32(portOrDefault(props.ServerPort, defaultAPIPort))
	metricsPort := intstr.FromInt32(portOrDefault(props.PrometheusPort, defaultPrometheusPort))
	postgresPort := intstr.FromInt32(portOrDefault(props.DBPort, defaultDBPort))
	tcp := corev1.ProtocolTCP

	// DB egress is port-only (no pod/CIDR peer). NetworkPolicy cannot match on
	// hostname/URL, so restricting To in-cluster postgres would break external DB
	// deployments. Any destination on db_port is allowed; for in-cluster postgres,
	// results-postgres ingress still limits who may connect.
	dbEgress := networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &tcp, Port: &postgresPort},
		},
	}

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "results-api"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-results-api"},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						// Console Plugin, CLI, routes, watcher, and other clients
						// live in many namespaces — allow from all namespaces.
						From: []networkingv1.NetworkPolicyPeer{
							{NamespaceSelector: &metav1.LabelSelector{}},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &tcp, Port: &apiPort},
						},
					},
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					dbEgress,
					// Auth token review / impersonation talks to the API server.
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "results-watcher"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-results-watcher"},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkpolicy.PrometheusIngressRule(params, metricsPort),
				},
				// Keep an explicit DNS egress rule in addition to allow-all so DNS
				// keeps working if the API-server egress rule is later tightened.
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "results-retention-policy-agent"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "tekton-results-retention-policy-agent"},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					dbEgress,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "results-postgres"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": "tekton-results-postgres",
					},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "tekton-results-api"},
								},
							},
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "tekton-results-retention-policy-agent"},
								},
							},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &tcp, Port: &postgresPort},
						},
					},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
				},
			},
		},
	}
}

// portOrDefault returns *p as int32 when set to a valid TCP/UDP port (1–65535),
// otherwise def. Values outside that range (e.g. 99999) would produce NetworkPolicy
// objects Kubernetes rejects.
func portOrDefault(p *int64, def int32) int32 {
	if p != nil && *p >= 1 && *p <= 65535 {
		return int32(*p)
	}
	return def
}

// defaultDenyPolicy scopes deny-all to Results pods only.
// Do not reuse "tekton-default-deny" — Triggers already owns that name.
// Pod templates use app.kubernetes.io/name (not part-of) on the pod itself.
func defaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy(
		"results-default-deny",
		metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app.kubernetes.io/name",
					Operator: metav1.LabelSelectorOpIn,
					Values: []string{
						"tekton-results-api",
						"tekton-results-watcher",
						"tekton-results-retention-policy-agent",
						"tekton-results-postgres",
					},
				},
			},
		},
	)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, tr *v1alpha1.TektonResult) error {
	if tr.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, "results-network-policies")
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		resultsDefaultPolicies(r.platformParams, tr.Spec.ResultsAPIProperties)...,
	)
	manifest, err := networkpolicy.Generate(
		tr.Spec.NetworkPolicy,
		tr.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, tr, "results-network-policies", &manifest, passthroughTransform, nil)
}

func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = passthroughTransform
