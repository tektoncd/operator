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
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestChainsControllerDefaultPolicies_Count(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policies := chainsControllerDefaultPolicies(params)
	if got := len(policies); got != 1 {
		t.Fatalf("expected 1 policy, got %d", got)
	}
	if policies[0].Name != "chains-controller" {
		t.Errorf("expected policy name %q, got %q", "chains-controller", policies[0].Name)
	}
}

func TestChainsControllerDefaultPolicies_PolicyTypes(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policy := chainsControllerDefaultPolicies(params)[0]

	if len(policy.Spec.PolicyTypes) != 2 {
		t.Fatalf("expected 2 policy types, got %d", len(policy.Spec.PolicyTypes))
	}
	hasIngress, hasEgress := false, false
	for _, pt := range policy.Spec.PolicyTypes {
		if pt == networkingv1.PolicyTypeIngress {
			hasIngress = true
		}
		if pt == networkingv1.PolicyTypeEgress {
			hasEgress = true
		}
	}
	if !hasIngress || !hasEgress {
		t.Errorf("expected both Ingress and Egress policy types, got %v", policy.Spec.PolicyTypes)
	}
}

func TestChainsControllerDefaultPolicies_Ingress(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policy := chainsControllerDefaultPolicies(params)[0]

	if len(policy.Spec.Ingress) != 1 {
		t.Fatalf("expected 1 ingress rule (Prometheus), got %d", len(policy.Spec.Ingress))
	}
	rule := policy.Spec.Ingress[0]
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != 9090 {
		t.Errorf("expected ingress port 9090, got %v", rule.Ports)
	}
}

func TestChainsControllerDefaultPolicies_EgressUnrestricted(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policy := chainsControllerDefaultPolicies(params)[0]

	if len(policy.Spec.Egress) != 1 {
		t.Fatalf("expected 1 egress rule (unrestricted), got %d", len(policy.Spec.Egress))
	}
	rule := policy.Spec.Egress[0]
	if len(rule.Ports) != 0 {
		t.Errorf("expected no port restriction (unrestricted egress), got %v", rule.Ports)
	}
	if len(rule.To) != 0 {
		t.Errorf("expected no destination restriction (unrestricted egress), got %v", rule.To)
	}
}

func TestChainsControllerDefaultPolicies_PodSelector(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	policy := chainsControllerDefaultPolicies(params)[0]

	labels := policy.Spec.PodSelector.MatchLabels
	expected := map[string]string{
		"app.kubernetes.io/name":      "controller",
		"app.kubernetes.io/component": "controller",
		"app.kubernetes.io/instance":  "default",
		"app.kubernetes.io/part-of":   "tekton-chains",
	}
	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("expected label %s=%s, got %s=%s", k, v, k, labels[k])
		}
	}
}

func TestChainsControllerDefaultDenyPolicy(t *testing.T) {
	policy := chainsControllerDefaultDenyPolicy()

	if policy.Name != "chains-controller-default-deny" {
		t.Errorf("expected name %q, got %q", "chains-controller-default-deny", policy.Name)
	}
	if len(policy.Spec.Ingress) != 0 {
		t.Errorf("expected no ingress rules (deny all), got %d", len(policy.Spec.Ingress))
	}
	if len(policy.Spec.Egress) != 0 {
		t.Errorf("expected no egress rules (deny all), got %d", len(policy.Spec.Egress))
	}
	labels := policy.Spec.PodSelector.MatchLabels
	if labels["app.kubernetes.io/part-of"] != "tekton-chains" {
		t.Errorf("expected pod selector for tekton-chains, got %v", labels)
	}
}

func TestChainsControllerDefaultPolicies_OpenShift(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	policy := chainsControllerDefaultPolicies(params)[0]

	rule := policy.Spec.Ingress[0]
	if len(rule.From) != 1 || rule.From[0].NamespaceSelector == nil {
		t.Fatalf("expected Prometheus ingress with NamespaceSelector, got %v", rule.From)
	}
	nsLabels := rule.From[0].NamespaceSelector.MatchLabels
	if nsLabels["openshift.io/cluster-monitoring"] != "true" {
		t.Errorf("expected OpenShift Prometheus namespace selector, got %v", nsLabels)
	}
}
