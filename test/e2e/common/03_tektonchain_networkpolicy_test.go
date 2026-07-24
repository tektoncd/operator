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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestTektonChainNetworkPolicy verifies NetworkPolicies are created by default
// for the Chains controller workload, that the controller keeps working under
// those policies (a TaskRun completes and Chains can reach the API server to
// observe it), and that toggling spec.networkPolicy.disabled correctly adds
// and removes the policies.
func TestTektonChainNetworkPolicy(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	// Ensure TektonConfig exists and is Ready. On OpenShift the operator
	// auto-creates it; on Kubernetes we create it if absent.
	// for a NetworkPolicy-focused test.
	if _, err := clients.TektonConfig().Get(context.TODO(), crNames.TektonConfig, metav1.GetOptions{}); err != nil {
		tc := &v1alpha1.TektonConfig{
			ObjectMeta: metav1.ObjectMeta{Name: crNames.TektonConfig},
			Spec: v1alpha1.TektonConfigSpec{
				Profile: v1alpha1.ProfileAll,
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: crNames.TargetNamespace,
				},
			},
		}
		if _, err := clients.TektonConfig().Create(context.TODO(), tc, metav1.CreateOptions{}); err != nil {
			t.Fatalf("TektonConfig %q failed to create: %v", crNames.TektonConfig, err)
		}
	}
	resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)

	// TektonConfig creates TektonChain as a child CR; wait for it.
	resources.AssertTektonChainCRReadyStatus(t, clients, crNames)

	expectedPolicies := []string{
		"chains-controller-default-deny",
		"chains-controller",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	// A successful TaskRun proves the Chains controller is functional under
	// NetworkPolicies: the controller watches the API server for TaskRun
	// completions (egress to API server must work) and Prometheus scrapes
	// metrics (ingress on port 9090). If the NetworkPolicy blocked API server
	// access, the controller would crash-loop.
	t.Run("chains-controller-functional-with-networkpolicies", func(t *testing.T) {
		taskRun := createChainNPProbeTaskRun(crNames.TargetNamespace)
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
		// Update TektonConfig (not TektonChain directly) because TektonConfig
		// owns TektonChain and propagates its NetworkPolicy on every reconcile.
		cfg, err := clients.TektonConfig().Get(context.TODO(), crNames.TektonConfig, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonConfig: %v", err)
		}
		cfg.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.TektonConfig().Update(context.TODO(), cfg, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on TektonConfig: %v", err)
		}
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		cfg, err := clients.TektonConfig().Get(context.TODO(), crNames.TektonConfig, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonConfig: %v", err)
		}
		cfg.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.TektonConfig().Update(context.TODO(), cfg, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on TektonConfig: %v", err)
		}
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}

// createChainNPProbeTaskRun creates a minimal TaskRun that completes quickly.
// Its completion exercises the Chains controller's API server watch (egress)
// since Chains observes TaskRun completions to sign and attest artifacts.
func createChainNPProbeTaskRun(namespace string) *pipelinev1.TaskRun {
	return &pipelinev1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "chain-np-probe-taskrun-",
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
						Args:    []string{"chains controller NetworkPolicy probe"},
					},
				},
			},
		},
	}
}
