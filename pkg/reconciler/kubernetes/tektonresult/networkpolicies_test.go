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
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/ptr"
)

func TestResultsDefaultPolicies(t *testing.T) {
	policies := resultsDefaultPolicies(networkpolicy.OpenShiftPlatformDefaults(), v1alpha1.ResultsAPIProperties{})
	wantNames := []string{
		"results-api",
		"results-watcher",
		"results-retention-policy-agent",
		"results-postgres",
	}
	if len(policies) != len(wantNames) {
		t.Fatalf("expected %d policies, got %d", len(wantNames), len(policies))
	}
	byName := map[string]networkingv1.NetworkPolicy{}
	for i, name := range wantNames {
		if policies[i].Name != name {
			t.Errorf("policy[%d]: expected %q, got %q", i, name, policies[i].Name)
		}
		byName[policies[i].Name] = policies[i]
	}

	api := byName["results-api"]
	assertIngressHasPort(t, "results-api", api.Spec.Ingress, 8080)
	assertIngressHasPort(t, "results-api", api.Spec.Ingress, 9090)
	assertEgressHasDBPort(t, "results-api", api.Spec.Egress, 5432)

	watcher := byName["results-watcher"]
	if got := len(watcher.Spec.Egress); got != 2 {
		t.Fatalf("results-watcher: expected 2 egress rules (DNS + allow-all), got %d", got)
	}
	assertEgressHasDNS(t, "results-watcher", watcher.Spec.Egress, 5353)
	assertEgressHasAllowAll(t, "results-watcher", watcher.Spec.Egress)
	assertIngressHasPort(t, "results-watcher", watcher.Spec.Ingress, 9090)

	retention := byName["results-retention-policy-agent"]
	assertEgressHasDNS(t, "results-retention-policy-agent", retention.Spec.Egress, 5353)
	assertEgressHasDBPort(t, "results-retention-policy-agent", retention.Spec.Egress, 5432)

	postgres := byName["results-postgres"]
	assertIngressHasPort(t, "results-postgres", postgres.Spec.Ingress, 5432)
	assertIngressFromApp(t, "results-postgres", postgres.Spec.Ingress, "tekton-results-api")
	assertIngressFromApp(t, "results-postgres", postgres.Spec.Ingress, "tekton-results-retention-policy-agent")
}

func TestResultsDefaultPoliciesUsesSpecPorts(t *testing.T) {
	props := v1alpha1.ResultsAPIProperties{
		ServerPort:     ptr.Int64(18080),
		PrometheusPort: ptr.Int64(19090),
		DBPort:         ptr.Int64(15432),
	}
	policies := resultsDefaultPolicies(networkpolicy.KubernetesPlatformDefaults(), props)
	byName := map[string]networkingv1.NetworkPolicy{}
	for _, p := range policies {
		byName[p.Name] = p
	}

	assertIngressHasPort(t, "results-api", byName["results-api"].Spec.Ingress, 18080)
	assertIngressHasPort(t, "results-api", byName["results-api"].Spec.Ingress, 19090)
	assertEgressHasDBPort(t, "results-api", byName["results-api"].Spec.Egress, 15432)
	assertIngressHasPort(t, "results-watcher", byName["results-watcher"].Spec.Ingress, 19090)
	assertEgressHasDBPort(t, "results-retention-policy-agent", byName["results-retention-policy-agent"].Spec.Egress, 15432)
	assertIngressHasPort(t, "results-postgres", byName["results-postgres"].Spec.Ingress, 15432)
}

func TestPortOrDefault(t *testing.T) {
	if got := portOrDefault(nil, 8080); got != 8080 {
		t.Errorf("nil: got %d", got)
	}
	if got := portOrDefault(ptr.Int64(9090), 8080); got != 9090 {
		t.Errorf("set: got %d", got)
	}
	if got := portOrDefault(ptr.Int64(0), 8080); got != 8080 {
		t.Errorf("zero: got %d, want default", got)
	}
	if got := portOrDefault(ptr.Int64(99999), 5432); got != 5432 {
		t.Errorf("above max port: got %d, want default", got)
	}
	if got := portOrDefault(ptr.Int64(65535), 5432); got != 65535 {
		t.Errorf("max valid port: got %d", got)
	}
}

func TestResultsDefaultDenyPolicy(t *testing.T) {
	deny := defaultDenyPolicy()
	if deny.Name != "results-default-deny" {
		t.Fatalf("expected name results-default-deny, got %q", deny.Name)
	}
}

func TestGenerateResultsNetworkPolicies(t *testing.T) {
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		resultsDefaultPolicies(networkpolicy.KubernetesPlatformDefaults(), v1alpha1.ResultsAPIProperties{})...,
	)
	m, err := networkpolicy.Generate(v1alpha1.NetworkPolicyConfig{}, "tekton-pipelines", defaults)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got := len(m.Resources()); got != 5 {
		t.Errorf("expected 5 resources (deny + 4 defaults), got %d", got)
	}

	disabled, err := networkpolicy.Generate(
		v1alpha1.NetworkPolicyConfig{Disabled: true},
		"tekton-pipelines",
		defaults,
	)
	if err != nil {
		t.Fatalf("Generate disabled: %v", err)
	}
	if got := len(disabled.Resources()); got != 0 {
		t.Errorf("expected empty manifest when disabled, got %d", got)
	}
}

func assertIngressHasPort(t *testing.T, policy string, rules []networkingv1.NetworkPolicyIngressRule, port int32) {
	t.Helper()
	want := intstr.FromInt32(port)
	for _, rule := range rules {
		for _, p := range rule.Ports {
			if p.Port != nil && *p.Port == want {
				return
			}
		}
	}
	t.Errorf("%s: expected ingress port %d", policy, port)
}

// assertEgressHasDBPort checks for a port-only DB egress rule (no To peer), so
// in-cluster and external databases both work without a custom NetworkPolicy.
func assertEgressHasDBPort(t *testing.T, policy string, rules []networkingv1.NetworkPolicyEgressRule, port int32) {
	t.Helper()
	want := intstr.FromInt32(port)
	for _, rule := range rules {
		if len(rule.To) != 0 {
			continue
		}
		for _, p := range rule.Ports {
			if p.Port != nil && *p.Port == want {
				return
			}
		}
	}
	t.Errorf("%s: expected port-only egress TCP/%d (no To peer)", policy, port)
}

func assertEgressHasDNS(t *testing.T, policy string, rules []networkingv1.NetworkPolicyEgressRule, dnsPort int32) {
	t.Helper()
	want := intstr.FromInt32(dnsPort)
	for _, rule := range rules {
		if len(rule.To) == 0 {
			continue
		}
		hasUDP, hasTCP := false, false
		for _, p := range rule.Ports {
			if p.Port == nil || *p.Port != want || p.Protocol == nil {
				continue
			}
			switch *p.Protocol {
			case corev1.ProtocolUDP:
				hasUDP = true
			case corev1.ProtocolTCP:
				hasTCP = true
			}
		}
		if hasUDP && hasTCP {
			return
		}
	}
	t.Errorf("%s: expected DNS egress rule with UDP+TCP/%d", policy, dnsPort)
}

func assertEgressHasAllowAll(t *testing.T, policy string, rules []networkingv1.NetworkPolicyEgressRule) {
	t.Helper()
	for _, rule := range rules {
		if len(rule.Ports) == 0 && len(rule.To) == 0 {
			return
		}
	}
	t.Errorf("%s: expected allow-all egress rule", policy)
}

func assertIngressFromApp(t *testing.T, policy string, rules []networkingv1.NetworkPolicyIngressRule, app string) {
	t.Helper()
	for _, rule := range rules {
		for _, peer := range rule.From {
			if peer.PodSelector != nil && peer.PodSelector.MatchLabels["app"] == app {
				return
			}
		}
	}
	t.Errorf("%s: expected ingress From app=%s", policy, app)
}
