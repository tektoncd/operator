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
	networkingv1 "k8s.io/api/networking/v1"
)

func TestResultsDefaultPolicies(t *testing.T) {
	policies := resultsDefaultPolicies(networkpolicy.OpenShiftPlatformDefaults())
	wantNames := []string{
		"results-api",
		"results-watcher",
		"results-retention-policy-agent",
		"results-postgres",
	}
	if len(policies) != len(wantNames) {
		t.Fatalf("expected %d policies, got %d", len(wantNames), len(policies))
	}
	for i, name := range wantNames {
		if policies[i].Name != name {
			t.Errorf("policy[%d]: expected %q, got %q", i, name, policies[i].Name)
		}
	}

	watcher := policies[1]
	if got := len(watcher.Spec.Egress); got != 1 {
		t.Fatalf("results-watcher: expected 1 egress rule (allow-all), got %d", got)
	}
	if len(watcher.Spec.Egress[0].Ports) != 0 || len(watcher.Spec.Egress[0].To) != 0 {
		t.Errorf("results-watcher: expected empty allow-all egress rule, got %#v", watcher.Spec.Egress[0])
	}
}

func TestGenerateResultsNetworkPolicies(t *testing.T) {
	defaults := append(
		[]networkingv1.NetworkPolicy{defaultDenyPolicy()},
		resultsDefaultPolicies(networkpolicy.KubernetesPlatformDefaults())...,
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
