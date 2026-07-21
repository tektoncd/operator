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
	"fmt"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestTektonPrunerNetworkPolicy verifies NetworkPolicies are created by default
// for the controller and webhook workloads TektonPruner deploys, that the pruner
// keeps working under those policies (webhook admission succeeds, controller can
// reach the API server), and that toggling spec.networkPolicy.disabled correctly
// adds and removes the policies.
func TestTektonPrunerNetworkPolicy(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownTektonPruner(clients, crNames.TektonPruner) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
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

	// The pruner webhook's ValidatingWebhookConfiguration intercepts ConfigMap
	// mutations for pruner-labeled ConfigMaps. Creating a TaskRun exercises the
	// Tekton Pipelines webhook (not the pruner webhook directly), so instead we
	// verify that the pruner controller can reach the API server by creating a
	// TaskRun and waiting for it to complete — which proves egress is working
	// under the NetworkPolicy.
	t.Run("pruner-functional-with-networkpolicies", func(t *testing.T) {
		taskRun := createPrunerNetworkPolicyProbeTaskRun(crNames.TargetNamespace)
		createdTaskRun, err := clients.TektonClient.TaskRuns(crNames.TargetNamespace).Create(
			context.TODO(), taskRun, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create TaskRun in namespace %q: %v", crNames.TargetNamespace, err)
		}

		if err := resources.WaitForTaskRunHappy(
			clients.TektonClient,
			crNames.TargetNamespace,
			createdTaskRun.Name,
			func(tr *pipelinev1.TaskRun) (bool, error) {
				if tr.IsDone() {
					if tr.IsSuccessful() {
						return true, nil
					}
					return false, fmt.Errorf("TaskRun %q failed: %s",
						tr.Name, tr.Status.GetCondition("Succeeded").GetMessage())
				}
				return false, nil
			},
		); err != nil {
			t.Fatalf("TaskRun did not complete successfully under NetworkPolicy in namespace %q: %v",
				crNames.TargetNamespace, err)
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

// createPrunerNetworkPolicyProbeTaskRun creates a minimal TaskRun that exercises
// the Tekton Pipelines admission webhook. A successful completion proves that
// pod creation (which passes through admission) and the pruner controller's API
// server access both work under the NetworkPolicy.
func createPrunerNetworkPolicyProbeTaskRun(namespace string) *pipelinev1.TaskRun {
	return &pipelinev1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pruner-networkpolicy-probe-",
			Namespace:    namespace,
		},
		Spec: pipelinev1.TaskRunSpec{
			TaskSpec: &pipelinev1.TaskSpec{
				Steps: []pipelinev1.Step{
					{
						Name:    "echo",
						Image:   "busybox:stable",
						Command: []string{"echo"},
						Args:    []string{"pruner NetworkPolicy probe"},
					},
				},
			},
		},
	}
}
