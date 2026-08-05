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

package syncerservice

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const syncerServiceCustomSet = "syncer-service-network-policies"

var syncerServicePodSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{"app": "workload-controller"},
}

func syncerServiceDefaultPolicies(params networkpolicy.PlatformParams) []networkingv1.NetworkPolicy {
	return []networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "syncer-service-controller"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: syncerServicePodSelector,
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					networkpolicy.DNSEgressRule(params),
					networkpolicy.APIServerEgressRule(),
				},
			},
		},
	}
}

func syncerServiceDefaultDenyPolicy() networkingv1.NetworkPolicy {
	return networkpolicy.DefaultDenyPolicy("syncer-service-default-deny", syncerServicePodSelector)
}

func (r *Reconciler) reconcileNetworkPolicies(ctx context.Context, ss *v1alpha1.SyncerService) error {
	if ss.Spec.NetworkPolicy.Disabled {
		return r.installerSetClient.CleanupCustomSet(ctx, syncerServiceCustomSet)
	}
	defaults := []networkingv1.NetworkPolicy{
		syncerServiceDefaultDenyPolicy(),
	}
	defaults = append(defaults, syncerServiceDefaultPolicies(r.platformParams)...)

	manifest, err := networkpolicy.Generate(
		ss.Spec.NetworkPolicy,
		ss.Spec.GetTargetNamespace(),
		defaults,
	)
	if err != nil {
		return err
	}
	return r.installerSetClient.CustomSet(ctx, ss, syncerServiceCustomSet, &manifest, passthroughTransform, nil)
}

func passthroughTransform(_ context.Context, m *mf.Manifest, _ v1alpha1.TektonComponent) (*mf.Manifest, error) {
	return m, nil
}

var _ client.FilterAndTransform = passthroughTransform
