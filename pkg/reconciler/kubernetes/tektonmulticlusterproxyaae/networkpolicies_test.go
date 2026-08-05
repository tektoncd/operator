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
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestConstants(t *testing.T) {
	if proxyAAECustomSet != "proxy-aae-network-policies" {
		t.Errorf("expected custom set proxy-aae-network-policies, got %q", proxyAAECustomSet)
	}
}

func TestProxyAAEDefaultPolicies(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policies := proxyAAEDefaultPolicies(params)

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	p := policies[0]

	if p.Name != "proxy-aae" {
		t.Errorf("expected name proxy-aae, got %q", p.Name)
	}

	labels := p.Spec.PodSelector.MatchLabels
	if labels["app"] != "proxy-aae" {
		t.Errorf("expected app=proxy-aae, got %q", labels["app"])
	}

	if len(p.Spec.PolicyTypes) != 2 {
		t.Fatalf("expected 2 policy types, got %d", len(p.Spec.PolicyTypes))
	}
	if p.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress || p.Spec.PolicyTypes[1] != networkingv1.PolicyTypeEgress {
		t.Errorf("expected [Ingress, Egress], got %v", p.Spec.PolicyTypes)
	}

	// Ingress: TCP 8080 from any
	if len(p.Spec.Ingress) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(p.Spec.Ingress))
	}
	ingressPorts := p.Spec.Ingress[0].Ports
	if len(ingressPorts) != 1 {
		t.Fatalf("expected 1 ingress port, got %d", len(ingressPorts))
	}
	if ingressPorts[0].Port.IntVal != 8080 {
		t.Errorf("expected ingress port 8080, got %d", ingressPorts[0].Port.IntVal)
	}
	if *ingressPorts[0].Protocol != corev1.ProtocolTCP {
		t.Errorf("expected TCP protocol, got %v", *ingressPorts[0].Protocol)
	}
	if len(p.Spec.Ingress[0].From) != 0 {
		t.Errorf("expected ingress from any (no From restriction), got %v", p.Spec.Ingress[0].From)
	}

	// Egress: DNS + API server
	if len(p.Spec.Egress) != 2 {
		t.Fatalf("expected 2 egress rules (DNS + API server), got %d", len(p.Spec.Egress))
	}
}

func TestProxyAAEDefaultDenyPolicy(t *testing.T) {
	p := proxyAAEDefaultDenyPolicy()

	if p.Name != "proxy-aae-default-deny" {
		t.Errorf("expected name proxy-aae-default-deny, got %q", p.Name)
	}
	if p.Spec.PodSelector.MatchLabels["app"] != "proxy-aae" {
		t.Errorf("expected deny scoped to app=proxy-aae, got %v", p.Spec.PodSelector.MatchLabels)
	}
}

func TestProxyAAEPolicies_OpenShift(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	policies := proxyAAEDefaultPolicies(params)

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
