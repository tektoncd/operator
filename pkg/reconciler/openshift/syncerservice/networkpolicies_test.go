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
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestConstants(t *testing.T) {
	if syncerServiceCustomSet != "syncer-service-network-policies" {
		t.Errorf("expected custom set syncer-service-network-policies, got %q", syncerServiceCustomSet)
	}
}

func TestSyncerServiceDefaultPolicies(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	policies := syncerServiceDefaultPolicies(params)

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	p := policies[0]

	if p.Name != "syncer-service-controller" {
		t.Errorf("expected name syncer-service-controller, got %q", p.Name)
	}

	labels := p.Spec.PodSelector.MatchLabels
	if labels["app"] != "workload-controller" {
		t.Errorf("expected app=workload-controller, got %q", labels["app"])
	}

	// Egress-only: no ingress rules
	if len(p.Spec.PolicyTypes) != 1 {
		t.Fatalf("expected 1 policy type (Egress only), got %d", len(p.Spec.PolicyTypes))
	}
	if p.Spec.PolicyTypes[0] != networkingv1.PolicyTypeEgress {
		t.Errorf("expected PolicyTypeEgress, got %v", p.Spec.PolicyTypes[0])
	}
	if len(p.Spec.Ingress) != 0 {
		t.Errorf("expected no ingress rules, got %d", len(p.Spec.Ingress))
	}

	// Egress: DNS + API server
	if len(p.Spec.Egress) != 2 {
		t.Fatalf("expected 2 egress rules (DNS + API server), got %d", len(p.Spec.Egress))
	}

	// Verify OpenShift DNS port 5353
	egressDNS := p.Spec.Egress[0]
	if len(egressDNS.Ports) < 1 {
		t.Fatal("expected DNS egress ports")
	}
	if egressDNS.Ports[0].Port.IntVal != 5353 {
		t.Errorf("expected OpenShift DNS port 5353, got %d", egressDNS.Ports[0].Port.IntVal)
	}
}

func TestSyncerServiceDefaultDenyPolicy(t *testing.T) {
	p := syncerServiceDefaultDenyPolicy()

	if p.Name != "syncer-service-default-deny" {
		t.Errorf("expected name syncer-service-default-deny, got %q", p.Name)
	}
	if p.Spec.PodSelector.MatchLabels["app"] != "workload-controller" {
		t.Errorf("expected deny scoped to app=workload-controller, got %v", p.Spec.PodSelector.MatchLabels)
	}
}
