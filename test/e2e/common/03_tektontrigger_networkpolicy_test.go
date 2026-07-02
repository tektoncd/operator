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
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	npTestNamespace     = "tekton-np-e2e"
	npEventListenerName = "np-test-listener"
)

// TestTektonTriggerNetworkPolicy verifies NetworkPolicies are created by default,
// that Triggers resources work correctly under those policies (EventListener
// receives events and creates PipelineRuns), and that toggling
// spec.networkPolicy.disabled correctly adds and removes the policies.
func TestTektonTriggerNetworkPolicy(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownTrigger(clients, crNames.TektonTrigger) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, npTestNamespace) })
	utils.CleanupOnInterrupt(func() { deleteNPTestClusterRoleBinding(t) })
	defer utils.TearDownNamespace(clients, npTestNamespace)
	defer deleteNPTestClusterRoleBinding(t)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownTrigger(clients, crNames.TektonTrigger)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}
	resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)

	if _, err := resources.EnsureTektonTriggerExists(clients.TektonTrigger(), crNames); err != nil {
		t.Fatalf("TektonTrigger %q failed to create: %v", crNames.TektonTrigger, err)
	}
	resources.AssertTektonTriggerCRReadyStatus(t, clients, crNames)

	expectedPolicies := []string{
		"tekton-default-deny",
		"triggers-controller",
		"triggers-webhook",
		"triggers-core-interceptors",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("triggers-functional-with-networkpolicies", func(t *testing.T) {
		if err := resources.CreateNamespace(clients.KubeClient, npTestNamespace); err != nil {
			t.Fatalf("failed to create test namespace %q: %v", npTestNamespace, err)
		}
		if err := applyTriggersTestdata(t, npTestNamespace); err != nil {
			t.Fatalf("failed to apply Triggers testdata: %v", err)
		}

		// EventListener Ready proves controller reached the API server.
		resources.AssertEventListenerReady(t, clients, npTestNamespace, npEventListenerName)

		// action=push matches the CEL filter; exercises interceptors ingress + API server egress.
		t.Run("cel-interceptor-matching-event-creates-pipelinerun", func(t *testing.T) {
			if err := sendEventToListener(t, npTestNamespace, npEventListenerName, `{"action":"push"}`); err != nil {
				t.Fatalf("matching event failed: %v", err)
			}
			resources.AssertPipelineRunCreated(t, clients, npTestNamespace)
		})

		// action=open does not match; interceptor blocks it, no new PipelineRun.
		t.Run("cel-interceptor-non-matching-event-blocked", func(t *testing.T) {
			prsBefore, err := clients.TektonClient.PipelineRuns(npTestNamespace).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("failed to list PipelineRuns: %v", err)
			}
			_ = sendEventToListener(t, npTestNamespace, npEventListenerName, `{"action":"open"}`)
			resources.AssertPipelineRunCountUnchanged(t, clients, npTestNamespace, len(prsBefore.Items))
		})
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		tt, err := clients.TektonTrigger().Get(context.TODO(), crNames.TektonTrigger, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonTrigger: %v", err)
		}
		tt.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.TektonTrigger().Update(context.TODO(), tt, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on TektonTrigger: %v", err)
		}
		resources.AssertTektonTriggerCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		tt, err := clients.TektonTrigger().Get(context.TODO(), crNames.TektonTrigger, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonTrigger: %v", err)
		}
		tt.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.TektonTrigger().Update(context.TODO(), tt, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on TektonTrigger: %v", err)
		}
		resources.AssertTektonTriggerCRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}

// deleteNPTestClusterRoleBinding removes the cluster-scoped binding created by rbac.yaml.
func deleteNPTestClusterRoleBinding(t *testing.T) {
	t.Helper()
	cmd := exec.Command("kubectl", "delete", "clusterrolebinding", "np-test-el-clusterrolebinding", "--ignore-not-found")
	_ = cmd.Run()
}

// applyTriggersTestdata applies testdata/triggers/ YAML into namespace.
func applyTriggersTestdata(t *testing.T, namespace string) error {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get caller information")
	}
	testdataDir := filepath.Join(filepath.Dir(file), "testdata", "triggers")

	var stderr bytes.Buffer
	cmd := exec.Command("kubectl", "apply", "-f", testdataDir, "-n", namespace)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %v\n%s", err, stderr.String())
	}
	return nil
}

// sendEventToListener POSTs payload to the EventListener from a temporary in-cluster pod.
func sendEventToListener(t *testing.T, namespace, listenerName, payload string) error {
	t.Helper()
	svcURL := fmt.Sprintf("http://el-%s.%s.svc.cluster.local:8080", listenerName, namespace)

	var stderr bytes.Buffer
	cmd := exec.Command(
		"kubectl", "run", "np-e2e-curl", "--restart=Never", "--rm", "-i",
		"--image=curlimages/curl:latest",
		"-n", namespace,
		"--",
		"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
		"-X", "POST", svcURL,
		"-H", "Content-Type: application/json",
		"-d", payload,
	)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("curl pod failed: %v\nstderr: %s", err, stderr.String())
	}
	// kubectl --rm appends the pod deletion message to stdout; take only the first
	// 3 bytes which are the curl %{http_code} output.
	raw := string(bytes.TrimSpace(out))
	if len(raw) < 3 || (raw[:3] != "200" && raw[:3] != "202") {
		return fmt.Errorf("EventListener returned unexpected HTTP status %q (want 200/202)", raw)
	}
	return nil
}
