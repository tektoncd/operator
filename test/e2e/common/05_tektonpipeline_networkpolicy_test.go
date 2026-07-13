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

// TestTektonPipelineNetworkPolicy verifies NetworkPolicies are created by default
// for the proxy-webhook workload TektonPipeline deploys, that the proxy-webhook
// keeps working under those policies (its mutating admission callback for TaskRun
// Pods still succeeds), and that toggling spec.networkPolicy.disabled correctly
// adds and removes the policies.
func TestTektonPipelineNetworkPolicy(t *testing.T) {
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

	expectedPolicies := []string{
		"tekton-proxy-webhook-default-deny",
		"proxy-webhook",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	// The proxy-webhook's MutatingWebhookConfiguration (failurePolicy: Fail) intercepts
	// Pod Create requests labeled app.kubernetes.io/managed-by=tekton-pipelines (see
	// pkg/reconciler/proxy/proxy.go). A TaskRun's underlying Pod carries that label, so
	// running one to completion proves the API server could reach the proxy-webhook on
	// port 8443 under the NetworkPolicy's ingress rule — if it couldn't, Pod admission
	// would be rejected and the TaskRun would never start.
	t.Run("proxy-webhook-functional-with-networkpolicies", func(t *testing.T) {
		taskRun := createNetworkPolicyProbeTaskRun(crNames.TargetNamespace)
		createdTaskRun, err := clients.TektonClient.TaskRuns(crNames.TargetNamespace).Create(
			context.TODO(), taskRun, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create TaskRun: %v", err)
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
					return false, fmt.Errorf("TaskRun failed")
				}
				return false, nil
			},
		); err != nil {
			t.Fatalf("TaskRun did not complete successfully under NetworkPolicy: %v", err)
		}
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		tp, err := clients.TektonPipeline().Get(context.TODO(), crNames.TektonPipeline, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonPipeline: %v", err)
		}
		tp.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.TektonPipeline().Update(context.TODO(), tp, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on TektonPipeline: %v", err)
		}
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		tp, err := clients.TektonPipeline().Get(context.TODO(), crNames.TektonPipeline, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonPipeline: %v", err)
		}
		tp.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.TektonPipeline().Update(context.TODO(), tp, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on TektonPipeline: %v", err)
		}
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}

// createNetworkPolicyProbeTaskRun creates a minimal TaskRun whose Pod triggers the
// proxy-webhook's mutating admission callback (see comment above).
func createNetworkPolicyProbeTaskRun(namespace string) *pipelinev1.TaskRun {
	return &pipelinev1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "networkpolicy-probe-taskrun-",
			Namespace:    namespace,
		},
		Spec: pipelinev1.TaskRunSpec{
			TaskSpec: &pipelinev1.TaskSpec{
				Steps: []pipelinev1.Step{
					{
						Name:    "echo",
						Image:   "busybox:stable",
						Command: []string{"echo"},
						Args:    []string{"proxy-webhook NetworkPolicy probe"},
					},
				},
			},
		},
	}
}
