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
	"os"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

func TestManualApprovalGateDeployment(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:       "config",
		TektonPipeline:     "pipeline",
		ManualApprovalGate: "manual-approval-gate",
		TargetNamespace:    "tekton-pipelines",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownManualApprovalGate(clients, crNames.ManualApprovalGate) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownManualApprovalGate(clients, crNames.ManualApprovalGate)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create ManualApprovalGate
	if _, err := resources.EnsureManualApprovalGateExists(clients.ManualApprovalGate(), crNames); err != nil {
		t.Fatalf("ManualApproval %q failed to create: %v", crNames.ManualApprovalGate, err)
	}

	t.Run("create-manual-approval-gate", func(t *testing.T) {
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-manual-approval-gate-deployments", func(t *testing.T) {
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyDeployments(t, clients, crNames.TargetNamespace, utils.ManualApprovalGateDeploymentLabel)
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
	})

	// Delete the ManualApprovalGate CR instance to see if all resource will be removed
	t.Run("delete-manual-approval-gate", func(t *testing.T) {
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
		resources.ManualApprovalGateCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
