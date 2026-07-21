//go:build e2e
// +build e2e

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

package common

import (
	"context"
	"strings"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const prunerNPTestNamespace = "pruner-np-e2e"

// TestTektonPrunerNetworkPolicy verifies NetworkPolicies are created by default
// for the controller and webhook workloads TektonPruner deploys, that the pruner
// webhook keeps working under those policies (ConfigMap validation via the
// pruner's own ValidatingWebhookConfiguration on port 8443 succeeds), and that
// toggling spec.networkPolicy.disabled correctly adds and removes the policies.
func TestTektonPrunerNetworkPolicy(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownTektonPruner(clients, crNames.TektonPruner) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, prunerNPTestNamespace) })
	defer utils.TearDownNamespace(clients, prunerNPTestNamespace)
	defer utils.TearDownTektonPruner(clients, crNames.TektonPruner)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}
	resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)

	if _, err := resources.EnsureTektonPrunerExists(clients.TektonPruner(), crNames); err != nil {
		t.Fatalf("TektonPruner %q failed to create: %v", crNames.TektonPruner, err)
	}
	resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)

	expectedPolicies := []string{
		"tekton-pruner-default-deny",
		"pruner-controller",
		"pruner-webhook",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	// The pruner's ValidatingWebhookConfiguration (failurePolicy: Fail) intercepts
	// ConfigMap mutations with labels app.kubernetes.io/part-of=tekton-pruner and
	// pruner.tekton.dev/config-type=namespace. Creating such a ConfigMap exercises the
	// pruner webhook on port 8443 — if the NetworkPolicy blocked ingress on that port,
	// the API server would reject the request because failurePolicy is Fail.
	t.Run("pruner-webhook-functional-with-networkpolicies", func(t *testing.T) {
		if err := resources.CreateNamespace(clients.KubeClient, prunerNPTestNamespace); err != nil {
			t.Fatalf("failed to create test namespace %q: %v", prunerNPTestNamespace, err)
		}

		// Valid ConfigMap — accepted by the pruner webhook (proves webhook reachable on 8443)
		validCM := prunerNamespaceConfigMap(prunerNPTestNamespace, "ttlSecondsAfterFinished: 600")
		_, err := clients.KubeClient.CoreV1().ConfigMaps(prunerNPTestNamespace).Create(
			context.TODO(), validCM, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("pruner webhook rejected valid ConfigMap in namespace %q: %v (NetworkPolicy may be blocking port 8443)",
				prunerNPTestNamespace, err)
		}

		// Invalid ConfigMap — rejected by the pruner webhook (proves active validation, not passthrough)
		invalidCM := prunerNamespaceConfigMap(prunerNPTestNamespace, "ttlSecondsAfterFinished: -999")
		invalidCM.Name = "tekton-pruner-namespace-spec-invalid"
		_, err = clients.KubeClient.CoreV1().ConfigMaps(prunerNPTestNamespace).Create(
			context.TODO(), invalidCM, metav1.CreateOptions{})
		if err == nil {
			t.Fatal("pruner webhook accepted invalid ConfigMap (negative TTL) — webhook may not be validating")
		}
		if !errors.IsForbidden(err) && !errors.IsInvalid(err) {
			if !strings.Contains(err.Error(), "denied the request") {
				t.Fatalf("unexpected error creating invalid ConfigMap: %v (expected webhook rejection)", err)
			}
		}
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		tp, err := clients.TektonPruner().Get(context.TODO(), crNames.TektonPruner, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonPruner %q: %v", crNames.TektonPruner, err)
		}
		tp.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.TektonPruner().Update(context.TODO(), tp, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on TektonPruner %q: %v", crNames.TektonPruner, err)
		}
		resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		tp, err := clients.TektonPruner().Get(context.TODO(), crNames.TektonPruner, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonPruner %q: %v", crNames.TektonPruner, err)
		}
		tp.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.TektonPruner().Update(context.TODO(), tp, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on TektonPruner %q: %v", crNames.TektonPruner, err)
		}
		resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}

// prunerNamespaceConfigMap creates a ConfigMap with the labels that trigger the
// pruner's ValidatingWebhookConfiguration (namespace-level webhook).
func prunerNamespaceConfigMap(namespace, configData string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tekton-pruner-namespace-spec",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of":     "tekton-pruner",
				"pruner.tekton.dev/config-type": "namespace",
			},
		},
		Data: map[string]string{
			"ns-config": configData,
		},
	}
}
