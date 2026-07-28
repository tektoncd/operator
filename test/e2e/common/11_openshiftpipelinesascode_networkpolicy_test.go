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
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestOpenShiftPipelinesAsCodeNetworkPolicy verifies NetworkPolicies are
// created by default when OpenShiftPipelinesAsCode is installed, and that
// toggling spec.networkPolicy.disabled correctly adds and removes the policies.
func TestOpenShiftPipelinesAsCodeNetworkPolicy(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}
	resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)

	if _, err := resources.EnsureOpenShiftPipelinesAsCodeExists(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("OpenShiftPipelinesAsCode %q failed to create: %v", crNames.OpenShiftPipelinesAsCode, err)
	}
	resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	defer resources.OpenShiftPipelinesAsCodeCRDelete(t, clients, crNames)

	expectedPolicies := []string{
		"pac-default-deny",
		"pac-controller",
		"pac-watcher",
		"pac-webhook",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		pac, err := clients.OpenShiftPipelinesAsCode().Get(context.TODO(), crNames.OpenShiftPipelinesAsCode, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get OpenShiftPipelinesAsCode: %v", err)
		}
		pac.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.OpenShiftPipelinesAsCode().Update(context.TODO(), pac, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on OpenShiftPipelinesAsCode: %v", err)
		}
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		pac, err := clients.OpenShiftPipelinesAsCode().Get(context.TODO(), crNames.OpenShiftPipelinesAsCode, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get OpenShiftPipelinesAsCode: %v", err)
		}
		pac.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.OpenShiftPipelinesAsCode().Update(context.TODO(), pac, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on OpenShiftPipelinesAsCode: %v", err)
		}
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}
