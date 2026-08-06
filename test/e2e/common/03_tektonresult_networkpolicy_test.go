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
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// TestTektonResultNetworkPolicy verifies NetworkPolicies are created by default
// for Results workloads, that Results stays Ready under those policies (API can
// reach Postgres; watcher can reach the API server), and that toggling
// spec.networkPolicy.disabled correctly adds and removes the policies.
func TestTektonResultNetworkPolicy(t *testing.T) {

	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownResult(clients, crNames.TektonResult) })
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownResult(clients, crNames.TektonResult)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}
	resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)

	// Operator creates default TLS + Postgres secrets when missing.
	if _, err := resources.EnsureTektonResultExists(clients.TektonResult(), crNames); err != nil {
		t.Fatalf("TektonResult %q failed to create: %v", crNames.TektonResult, err)
	}
	resources.AssertTektonResultCRReadyStatus(t, clients, crNames)

	expectedPolicies := []string{
		"results-default-deny",
		"results-api",
		"results-watcher",
		"results-retention-policy-agent",
		"results-postgres",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	// Ready status already proves API↔DB and watcher↔API-server under NP.
	// A successful TaskRun additionally exercises watcher API-server egress
	t.Run("results-functional-with-networkpolicies", func(t *testing.T) {
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)

		taskRun := createResultNPProbeTaskRun(crNames.TargetNamespace)
		createdTaskRun, err := clients.TektonClient.TaskRuns(crNames.TargetNamespace).Create(
			context.TODO(), taskRun, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create TaskRun: %v", err)
		}
		defer deleteResultNPProbeTaskRun(t, clients, crNames.TargetNamespace, createdTaskRun.Name)

		if err := resources.WaitForTaskRunHappy(
			clients.TektonClient,
			crNames.TargetNamespace,
			createdTaskRun.Name,
			func(tr *pipelinev1.TaskRun) (bool, error) {
				if tr.IsDone() {
					if tr.IsSuccessful() {
						return true, nil
					}
					return false, fmt.Errorf("TaskRun failed")
				}
				return false, nil
			},
		); err != nil {
			t.Fatalf("TaskRun did not complete successfully under NetworkPolicy: %v", err)
		}

		// Results must still be Ready after the TaskRun (watcher/API/DB path).
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		tr, err := clients.TektonResult().Get(context.TODO(), crNames.TektonResult, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonResult: %v", err)
		}
		tr.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.TektonResult().Update(context.TODO(), tr, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on TektonResult: %v", err)
		}
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		tr, err := clients.TektonResult().Get(context.TODO(), crNames.TektonResult, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonResult: %v", err)
		}
		tr.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.TektonResult().Update(context.TODO(), tr, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on TektonResult: %v", err)
		}
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}

// deleteResultNPProbeTaskRun deletes the probe TaskRun and waits until it is
// fully gone. Must run while TektonResult is still installed so the Results
// watcher can clear results.tekton.dev/taskrun.
func deleteResultNPProbeTaskRun(t *testing.T, clients *utils.Clients, namespace, name string) {
	t.Helper()
	ctx := context.Background()
	err := clients.TektonClient.TaskRuns(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		t.Fatalf("failed to delete probe TaskRun %q: %v", name, err)
	}
	if err := wait.PollUntilContextTimeout(ctx, utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		_, getErr := clients.TektonClient.TaskRuns(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(getErr) {
			return true, nil
		}
		return false, getErr
	}); err != nil {
		t.Fatalf("probe TaskRun %q was not fully deleted (Results finalizer may be stuck): %v", name, err)
	}
}

// createResultNPProbeTaskRun creates a minimal TaskRun that completes quickly.
// Completion exercises the Results watcher watch of the API server under NP.
func createResultNPProbeTaskRun(namespace string) *pipelinev1.TaskRun {
	return &pipelinev1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "result-np-probe-taskrun-",
			Namespace:    namespace,
		},
		Spec: pipelinev1.TaskRunSpec{
			ServiceAccountName: "default",
			TaskSpec: &pipelinev1.TaskSpec{
				Steps: []pipelinev1.Step{
					{
						Name:    "echo",
						Image:   "busybox:stable",
						Command: []string{"echo"},
						Args:    []string{"results NetworkPolicy probe"},
					},
				},
			},
		},
	}
}
