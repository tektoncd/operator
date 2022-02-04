// +build e2e

/*
Copyright 2020 The Tekton Authors

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

package kubernetes

import (
	"testing"

	"github.com/tektoncd/operator/test/utils"

	"github.com/tektoncd/operator/test/client"

	"github.com/tektoncd/operator/test/resources"
)

// TestTektonDashboardsDeployment verifies the TektonDashboards creation, deployment recreation, and TektonDashboards deletion.
func TestTektonDashboardsDeployment(t *testing.T) {
	clients := client.Setup(t)

	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonDashboard: "dashboard",
		TargetNamespace: "tekton-pipelines",
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)

	utils.CleanupOnInterrupt(func() { utils.TearDownDashboard(clients, crNames.TektonDashboard) })
	defer utils.TearDownDashboard(clients, crNames.TektonDashboard)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create a TektonDashboard
	if _, err := resources.EnsureTektonDashboardExists(clients.TektonDashboard(), crNames); err != nil {
		t.Fatalf("TektonDashboard %q failed to create: %v", crNames.TektonDashboard, err)
	}

	// Test if TektonDashboard can reach the READY status
	t.Run("create-dashboard", func(t *testing.T) {
		resources.AssertTektonDashboardCRReadyStatus(t, clients, crNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-dashboard-deployments", func(t *testing.T) {
		resources.AssertTektonDashboardCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyDeployments(t, clients, crNames.TargetNamespace, utils.TektonDashboardDeploymentLabel)
		resources.AssertTektonDashboardCRReadyStatus(t, clients, crNames)
	})

	// Test if TektonInstallerSets are created.
	t.Run("verify-dashboard-installersets", func(t *testing.T) {
		resources.AssertDashboardInstallerSets(t, clients)
	})

	// Delete the TektonDashboard CR instance to see if all resources will be removed
	t.Run("delete-dashboard", func(t *testing.T) {
		resources.AssertTektonDashboardCRReadyStatus(t, clients, crNames)
		resources.TektonDashboardCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})

}
