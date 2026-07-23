//go:build e2e
// +build e2e

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

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// TestManualApprovalGateNetworkPolicy verifies NetworkPolicies are created by
// default when ManualApprovalGate is installed, and that toggling
// spec.networkPolicy.disabled correctly adds and removes the policies.
func TestManualApprovalGateNetworkPolicy(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:       "config",
		TektonPipeline:     "pipeline",
		ManualApprovalGate: "manual-approval-gate",
		TargetNamespace:    "tekton-pipelines",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}
	platform := os.Getenv("PLATFORM")
	if platform == "linux/ppc64le" || platform == "linux/s390x" {
		t.Skipf("ManualApprovalGate is not available for %q", platform)
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownManualApprovalGate(clients, crNames.ManualApprovalGate) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownManualApprovalGate(clients, crNames.ManualApprovalGate)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}
	resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)

	if _, err := resources.EnsureManualApprovalGateExists(clients.ManualApprovalGate(), crNames); err != nil {
		t.Fatalf("ManualApprovalGate %q failed to create: %v", crNames.ManualApprovalGate, err)
	}
	resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)

	expectedPolicies := []string{
		"mag-default-deny",
		"mag-controller",
		"mag-webhook",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	// A PipelineRun referencing the ApprovalTask custom task
	// (https://github.com/openshift-pipelines/manual-approval-gate) exercises the
	// full MAG request path under the default policies: the Tekton Pipeline
	// controller creates a CustomRun, MAG's own controller reconciles it and
	// creates the backing ApprovalTask object via the API server (mag-controller
	// egress), and that CREATE is admitted by MAG's ValidatingWebhookConfiguration
	// — proving the API server can reach the mag-webhook pod on port 8443 under
	// the mag-webhook policy's ingress rule. If either path were blocked, the
	// ApprovalTask would never reach a status.state.
	t.Run("mag-functional-with-networkpolicies", func(t *testing.T) {
		prName := createApprovalTaskPipelineRun(t, crNames.TargetNamespace)
		defer func() {
			_ = exec.Command("kubectl", "delete", "pipelinerun", prName, "-n", crNames.TargetNamespace, "--ignore-not-found").Run()
		}()

		state := waitForApprovalTaskState(t, crNames.TargetNamespace, prName)
		if state == "" {
			t.Fatalf("ApprovalTask for PipelineRun %q reported an empty status.state", prName)
		}
		t.Logf("ApprovalTask for PipelineRun %q reached status.state %q under NetworkPolicy", prName, state)
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		mag, err := clients.ManualApprovalGate().Get(context.TODO(), crNames.ManualApprovalGate, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get ManualApprovalGate: %v", err)
		}
		mag.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.ManualApprovalGate().Update(context.TODO(), mag, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on ManualApprovalGate: %v", err)
		}
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		mag, err := clients.ManualApprovalGate().Get(context.TODO(), crNames.ManualApprovalGate, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get ManualApprovalGate: %v", err)
		}
		mag.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.ManualApprovalGate().Update(context.TODO(), mag, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on ManualApprovalGate: %v", err)
		}
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}

// approvalTaskPipelineRunYAML is a minimal PipelineRun with a single task
// referencing the ApprovalTask custom task. The ApprovalTask CRD and its
// controller/webhook belong to a separate module (openshift-pipelines/
// manual-approval-gate) that this repo does not vendor a typed client for, so
// it is created and polled via kubectl, matching the pattern used by the
// TektonTrigger NetworkPolicy e2e test.
const approvalTaskPipelineRunYAML = `
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: np-e2e-approval-
  namespace: %s
spec:
  pipelineSpec:
    tasks:
      - name: wait-for-approval
        taskRef:
          apiVersion: openshift-pipelines.org/v1alpha1
          kind: ApprovalTask
        params:
          - name: approvers
            value:
              - np-e2e-approver
          - name: numberOfApprovalsRequired
            value: "1"
          - name: description
            value: "NetworkPolicy e2e functional probe"
`

// createApprovalTaskPipelineRun creates the PipelineRun above and returns its
// generated name.
func createApprovalTaskPipelineRun(t *testing.T, namespace string) string {
	t.Helper()

	var stdout, stderr strings.Builder
	cmd := exec.Command("kubectl", "create", "-f", "-", "-o", "jsonpath={.metadata.name}")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(approvalTaskPipelineRunYAML, namespace))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create ApprovalTask PipelineRun: %v\nstderr: %s", err, stderr.String())
	}
	name := strings.TrimSpace(stdout.String())
	if name == "" {
		t.Fatalf("kubectl did not return a PipelineRun name, stderr: %s", stderr.String())
	}
	return name
}

// waitForApprovalTaskState polls until MAG's controller has created the
// ApprovalTask object backing pipelineRunName's CustomRun and that object
// reports a non-empty status.state, then returns that state. It fails the
// test if either the CustomRun or the ApprovalTask never materializes, which
// would indicate the controller or webhook path was blocked.
func waitForApprovalTaskState(t *testing.T, namespace, pipelineRunName string) string {
	t.Helper()

	var customRunName string
	if err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		out, err := exec.CommandContext(ctx, "kubectl", "get", "customruns.tekton.dev",
			"-n", namespace,
			"-l", "tekton.dev/pipelineRun="+pipelineRunName,
			"-o", "jsonpath={.items[0].metadata.name}",
		).Output()
		if err != nil {
			return false, nil
		}
		customRunName = strings.TrimSpace(string(out))
		return customRunName != "", nil
	}); err != nil {
		t.Fatalf("no CustomRun created for PipelineRun %q: %v", pipelineRunName, err)
	}

	var state string
	if err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		out, err := exec.CommandContext(ctx, "kubectl", "get", "approvaltasks.openshift-pipelines.org",
			customRunName, "-n", namespace, "-o", "jsonpath={.status.state}",
		).Output()
		if err != nil {
			// Not created yet — the controller's create request may still be
			// in flight through the admission webhook.
			return false, nil
		}
		state = strings.TrimSpace(string(out))
		return state != "", nil
	}); err != nil {
		t.Fatalf("ApprovalTask %q for PipelineRun %q never reached a status.state "+
			"(controller or mag-webhook may be unreachable under NetworkPolicy): %v",
			customRunName, pipelineRunName, err)
	}
	return state
}
