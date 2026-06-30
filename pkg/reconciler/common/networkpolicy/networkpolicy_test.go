/*
Copyright 2024 The Tekton Authors

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

package networkpolicy_test

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func defaultPolicies() []networkingv1.NetworkPolicy {
	return []networkingv1.NetworkPolicy{
		{ObjectMeta: metav1.ObjectMeta{Name: "policy-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "policy-b"}},
	}
}

func TestGenerate_Disabled(t *testing.T) {
	cfg := v1alpha1.NetworkPolicyConfig{Disabled: true}
	m, err := networkpolicy.Generate(cfg, "tekton-pipelines", defaultPolicies())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(m.Resources()); got != 0 {
		t.Errorf("expected empty manifest when disabled, got %d resources", got)
	}
}

func TestGenerate_DefaultsOnly(t *testing.T) {
	cfg := v1alpha1.NetworkPolicyConfig{}
	m, err := networkpolicy.Generate(cfg, "tekton-pipelines", defaultPolicies())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(m.Resources()); got != 2 {
		t.Errorf("expected 2 resources, got %d", got)
	}
	for _, r := range m.Resources() {
		if r.GetNamespace() != "tekton-pipelines" {
			t.Errorf("resource %q: expected namespace tekton-pipelines, got %q", r.GetName(), r.GetNamespace())
		}
		if r.GetKind() != "NetworkPolicy" {
			t.Errorf("resource %q: expected kind NetworkPolicy, got %q", r.GetName(), r.GetKind())
		}
		if r.GetAPIVersion() != "networking.k8s.io/v1" {
			t.Errorf("resource %q: expected apiVersion networking.k8s.io/v1, got %q", r.GetName(), r.GetAPIVersion())
		}
	}
}

func TestGenerate_UserOverridesDefault(t *testing.T) {
	cfg := v1alpha1.NetworkPolicyConfig{
		Policies: map[string]networkingv1.NetworkPolicySpec{
			"policy-a": {
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "override"},
				},
			},
		},
	}
	m, err := networkpolicy.Generate(cfg, "tekton-pipelines", defaultPolicies())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// policy-a overridden, policy-b kept — still 2 total
	if got := len(m.Resources()); got != 2 {
		t.Errorf("expected 2 resources, got %d", got)
	}
	for _, r := range m.Resources() {
		if r.GetName() != "policy-a" {
			continue
		}
		labels, _, _ := unstructured.NestedStringMap(r.Object, "spec", "podSelector", "matchLabels")
		if labels["app"] != "override" {
			t.Errorf("policy-a: expected override label, got %v", labels)
		}
	}
}

func TestGenerate_UserAddsNewPolicy(t *testing.T) {
	cfg := v1alpha1.NetworkPolicyConfig{
		Policies: map[string]networkingv1.NetworkPolicySpec{
			"policy-new": {},
		},
	}
	m, err := networkpolicy.Generate(cfg, "tekton-pipelines", defaultPolicies())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(m.Resources()); got != 3 {
		t.Errorf("expected 3 resources (2 defaults + 1 new), got %d", got)
	}
}

func TestGenerate_DeterministicOrder(t *testing.T) {
	cfg := v1alpha1.NetworkPolicyConfig{}
	defaults := []networkingv1.NetworkPolicy{
		{ObjectMeta: metav1.ObjectMeta{Name: "policy-c"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "policy-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "policy-b"}},
	}
	m, err := networkpolicy.Generate(cfg, "ns", defaults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resources := m.Resources()
	expected := []string{"policy-a", "policy-b", "policy-c"}
	for i, r := range resources {
		if r.GetName() != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], r.GetName())
		}
	}
}

func TestGenerate_EmptyDefaults(t *testing.T) {
	cfg := v1alpha1.NetworkPolicyConfig{}
	m, err := networkpolicy.Generate(cfg, "ns", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(m.Resources()); got != 0 {
		t.Errorf("expected 0 resources, got %d", got)
	}
}

func TestWebhookIngressRule_NoCIDR(t *testing.T) {
	port := intstr.FromInt32(8443)
	rule := networkpolicy.WebhookIngressRule("", port)
	if len(rule.From) != 0 {
		t.Errorf("expected no From restriction when cidr empty, got %v", rule.From)
	}
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != 8443 {
		t.Errorf("expected port 8443, got %v", rule.Ports)
	}
}

func TestWebhookIngressRule_WithCIDR(t *testing.T) {
	port := intstr.FromInt32(8443)
	rule := networkpolicy.WebhookIngressRule("10.0.1.0/24", port)
	if len(rule.From) != 1 || rule.From[0].IPBlock == nil {
		t.Fatalf("expected From with IPBlock, got %v", rule.From)
	}
	if rule.From[0].IPBlock.CIDR != "10.0.1.0/24" {
		t.Errorf("expected CIDR 10.0.1.0/24, got %q", rule.From[0].IPBlock.CIDR)
	}
}

func TestInternetEgressRule(t *testing.T) {
	rule := networkpolicy.InternetEgressRule()
	if len(rule.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(rule.Ports))
	}
	if rule.Ports[0].Port.IntVal != 80 || rule.Ports[1].Port.IntVal != 443 {
		t.Errorf("expected ports 80 and 443, got %v and %v", rule.Ports[0].Port.IntVal, rule.Ports[1].Port.IntVal)
	}
	if len(rule.To) != 0 {
		t.Errorf("expected no To restriction for internet egress, got %v", rule.To)
	}
}

func TestAPIServerEgressRule_Kubernetes(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	rule := networkpolicy.APIServerEgressRule(params)
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != 443 {
		t.Fatalf("expected port 443 for Kubernetes, got %v", rule.Ports)
	}
	if len(rule.To) != 0 {
		t.Errorf("expected no To restriction for API server egress, got %v", rule.To)
	}
}

func TestAPIServerEgressRule_OpenShift(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	rule := networkpolicy.APIServerEgressRule(params)
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != 6443 {
		t.Fatalf("expected port 6443 for OpenShift, got %v", rule.Ports)
	}
	if len(rule.To) != 0 {
		t.Errorf("expected no To restriction for API server egress, got %v", rule.To)
	}
}

func TestDNSEgressRule_Kubernetes(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	rule := networkpolicy.DNSEgressRule(params)
	if len(rule.Ports) != 2 {
		t.Fatalf("expected 2 ports (UDP+TCP 53), got %d", len(rule.Ports))
	}
	for _, p := range rule.Ports {
		if p.Port.IntVal != 53 {
			t.Errorf("expected DNS port 53 for Kubernetes, got %d", p.Port.IntVal)
		}
	}
	if len(rule.To) != 1 || rule.To[0].NamespaceSelector == nil {
		t.Fatalf("expected 1 To with NamespaceSelector, got %v", rule.To)
	}
	nsLabels := rule.To[0].NamespaceSelector.MatchLabels
	if nsLabels["kubernetes.io/metadata.name"] != "kube-system" {
		t.Errorf("expected kube-system namespace selector, got %v", nsLabels)
	}
}

func TestDNSEgressRule_OpenShift(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	rule := networkpolicy.DNSEgressRule(params)
	if len(rule.To) == 0 || rule.To[0].NamespaceSelector == nil {
		t.Fatalf("expected 1 To with NamespaceSelector, got %v", rule.To)
	}
	nsLabels := rule.To[0].NamespaceSelector.MatchLabels
	if nsLabels["kubernetes.io/metadata.name"] != "openshift-dns" {
		t.Errorf("expected openshift-dns namespace selector, got %v", nsLabels)
	}
	for _, p := range rule.Ports {
		if p.Port.IntVal != 5353 {
			t.Errorf("expected DNS port 5353 for OpenShift, got %d", p.Port.IntVal)
		}
	}
}

func TestPrometheusIngressRule(t *testing.T) {
	params := networkpolicy.KubernetesPlatformDefaults()
	port := intstr.FromInt32(9000)
	rule := networkpolicy.PrometheusIngressRule(params, port)
	if len(rule.From) != 1 || rule.From[0].NamespaceSelector == nil {
		t.Fatalf("expected 1 From with NamespaceSelector, got %v", rule.From)
	}
	nsLabels := rule.From[0].NamespaceSelector.MatchLabels
	if nsLabels["kubernetes.io/metadata.name"] != "monitoring" {
		t.Errorf("expected monitoring namespace, got %v", nsLabels)
	}
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != 9000 {
		t.Errorf("expected port 9000, got %v", rule.Ports)
	}
}

func TestPrometheusIngressRule_OpenShift(t *testing.T) {
	params := networkpolicy.OpenShiftPlatformDefaults()
	port := intstr.FromInt32(9000)
	rule := networkpolicy.PrometheusIngressRule(params, port)
	if len(rule.From) != 1 || rule.From[0].NamespaceSelector == nil {
		t.Fatalf("expected 1 From with NamespaceSelector, got %v", rule.From)
	}
	nsLabels := rule.From[0].NamespaceSelector.MatchLabels
	if nsLabels["openshift.io/cluster-monitoring"] != "true" {
		t.Errorf("expected openshift.io/cluster-monitoring: true, got %v", nsLabels)
	}
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntVal != 9000 {
		t.Errorf("expected port 9000, got %v", rule.Ports)
	}
}
