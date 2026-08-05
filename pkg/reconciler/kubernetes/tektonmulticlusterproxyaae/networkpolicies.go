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

package tektonmulticlusterproxyaae

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

const proxyAAECustomSet = "proxy-aae-network-policies"

var proxyAAEPodSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{"app": "proxy-aae"},
}

func proxyAAEDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	proxyPort := intstr.FromInt32(8080)
	tcp := corev1.ProtocolTCP

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "proxy-aae"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: proxyAAEPodSelector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{Protocol: &tcp, Port: &proxyPort},
						},
					},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
	}
}

func proxyAAEDefaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy("proxy-aae-default-deny", proxyAAEPodSelector)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, proxy *v1alpha1.TektonMulticlusterProxyAAE) error {
	if proxy.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, proxyAAECustomSet)
	}
	defaults := []networkingv1.NetworkPolicy{
		proxyAAEDefaultDenyPolicy(),
	}
	defaults = append(defaults, proxyAAEDefaultPolicies(r.platformParams)...)

	manifest, err := networkpolicy.Generate(
		proxy.Spec.NetworkPolicy,
		proxy.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, proxy, proxyAAECustomSet, &manifest, passthroughTransform, nil)
}

func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = passthroughTransform
