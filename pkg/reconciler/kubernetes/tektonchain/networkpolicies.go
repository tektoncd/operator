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

package tektonchain

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

// chainsControllerPodSelector matches the tekton-chains-controller Deployment's
// spec.selector.matchLabels shipped in the upstream release manifest.
var chainsControllerPodSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{
		"app.kubernetes.io/name":      "controller",
		"app.kubernetes.io/component": "controller",
		"app.kubernetes.io/instance":  "default",
		"app.kubernetes.io/part-of":   "tekton-chains",
	},
}

// chainsControllerDefaultPolicies returns the default NetworkPolicies for the
// Chains controller workload.
//
// Egress rules:
//   - DNS resolution via the platform DNS resolver
//   - Unrestricted egress for all other traffic (API server, OCI registries,
//     Sigstore, KMS providers, storage backends).
func chainsControllerDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	metricsPort := intstr.FromInt32(9090)

	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "chains-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: chainsControllerPodSelector,
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

// chainsControllerDefaultDenyPolicy returns a scoped default-deny for the
// Chains controller pod only.
func chainsControllerDefaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy("chains-controller-default-deny", chainsControllerPodSelector)
}

// reconcileNetworkPolicies reconciles every NetworkPolicy owned by TektonChain
// as a single CustomSet named "chain-network-policies".
func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, tc *v1alpha1.TektonChain) error {
	if tc.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, "chain-network-policies")
	}
	defaults := append(
		[]networkingv1.NetworkPolicy{chainsControllerDefaultDenyPolicy()},
		chainsControllerDefaultPolicies(r.platformParams)...,
	)
	manifest, err := networkpolicy.Generate(
		tc.Spec.NetworkPolicy,
		tc.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, tc, "chain-network-policies", &manifest, npPassthroughTransform, nil)
}

// npPassthroughTransform is a no-op FilterAndTransform used for pre-built
// manifests where namespace injection is already handled by Generate.
func npPassthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = npPassthroughTransform
