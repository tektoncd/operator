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
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestConstants(t *testing.T) {
	if schedulerCustomSet != "scheduler-network-policies" {
		t.Errorf("expected custom set scheduler-network-policies, got %q", schedulerCustomSet)
	}
}

func TestSchedulerControllerDefaultPolicies(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policies := schedulerControllerDefaultPolicies(params)

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	p := policies[0]

	if p.Name != "scheduler-controller" {
		t.Errorf("expected name scheduler-controller, got %q", p.Name)
	}

	labels := p.Spec.PodSelector.MatchLabels
	if labels["app.kubernetes.io/name"] != "tekton-kueue" {
		t.Errorf("expected app.kubernetes.io/name=tekton-kueue, got %q", labels["app.kubernetes.io/name"])
	}
	if labels["control-plane"] != "controller-manager" {
		t.Errorf("expected control-plane=controller-manager, got %q", labels["control-plane"])
	}

	if len(p.Spec.PolicyTypes) != 2 {
		t.Fatalf("expected 2 policy types, got %d", len(p.Spec.PolicyTypes))
	}
	if p.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress || p.Spec.PolicyTypes[1] != networkingv1.PolicyTypeEgress {
		t.Errorf("expected [Ingress, Egress], got %v", p.Spec.PolicyTypes)
	}

	if len(p.Spec.Ingress) != 1 {
		t.Fatalf("expected 1 ingress rule (Prometheus), got %d", len(p.Spec.Ingress))
	}
	if len(p.Spec.Ingress[0].Ports) != 1 || p.Spec.Ingress[0].Ports[0].Port.IntVal != 8443 {
		t.Errorf("expected Prometheus ingress on port 8443, got %v", p.Spec.Ingress[0].Ports)
	}

	if len(p.Spec.Egress) != 2 {
		t.Fatalf("expected 2 egress rules (DNS + API server), got %d", len(p.Spec.Egress))
	}
}

func TestSchedulerWebhookDefaultPolicies(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policies := schedulerWebhookDefaultPolicies(params)

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	p := policies[0]

	if p.Name != "scheduler-webhook" {
		t.Errorf("expected name scheduler-webhook, got %q", p.Name)
	}

	labels := p.Spec.PodSelector.MatchLabels
	if labels["app.kubernetes.io/name"] != "tekton-kueue-webhook" {
		t.Errorf("expected app.kubernetes.io/name=tekton-kueue-webhook, got %q", labels["app.kubernetes.io/name"])
	}
	if labels["control-plane"] != "controller-manager" {
		t.Errorf("expected control-plane=controller-manager, got %q", labels["control-plane"])
	}

	if len(p.Spec.Ingress) != 2 {
		t.Fatalf("expected 2 ingress rules (webhook + Prometheus), got %d", len(p.Spec.Ingress))
	}
	if p.Spec.Ingress[0].Ports[0].Port.IntVal != 9443 {
		t.Errorf("expected webhook ingress on port 9443, got %d", p.Spec.Ingress[0].Ports[0].Port.IntVal)
	}
	if p.Spec.Ingress[1].Ports[0].Port.IntVal != 8443 {
		t.Errorf("expected Prometheus ingress on port 8443, got %d", p.Spec.Ingress[1].Ports[0].Port.IntVal)
	}

	if len(p.Spec.Egress) != 2 {
		t.Fatalf("expected 2 egress rules (DNS + API server), got %d", len(p.Spec.Egress))
	}
}

func TestSchedulerDefaultDenyPolicies(t *testing.T) {
	policies := schedulerDefaultDenyPolicies()

	if len(policies) != 2 {
		t.Fatalf("expected 2 default-deny policies, got %d", len(policies))
	}

	ctrl := policies[0]
	if ctrl.Name != "scheduler-controller-default-deny" {
		t.Errorf("expected name scheduler-controller-default-deny, got %q", ctrl.Name)
	}
	if ctrl.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"] != "tekton-kueue" {
		t.Errorf("expected controller deny scoped to tekton-kueue, got %v", ctrl.Spec.PodSelector.MatchLabels)
	}

	wh := policies[1]
	if wh.Name != "scheduler-webhook-default-deny" {
		t.Errorf("expected name scheduler-webhook-default-deny, got %q", wh.Name)
	}
	if wh.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"] != "tekton-kueue-webhook" {
		t.Errorf("expected webhook deny scoped to tekton-kueue-webhook, got %v", wh.Spec.PodSelector.MatchLabels)
	}
}

func TestSchedulerControllerPolicies_OpenShift(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	policies := schedulerControllerDefaultPolicies(params)

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	egressDNS := policies[0].Spec.Egress[0]
	if len(egressDNS.Ports) < 1 {
		t.Fatal("expected DNS egress ports")
	}
	if egressDNS.Ports[0].Port.IntVal != 5353 {
		t.Errorf("expected OpenShift DNS port 5353, got %d", egressDNS.Ports[0].Port.IntVal)
	}
}
