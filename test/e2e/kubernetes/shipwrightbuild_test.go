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

package kubernetes

import (
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

// TestShipwrightBuildDeployment verifies the ShipwrightBuild creation, deployment recreation, and ShipwrightBuild deletion.
func TestShipwrightBuild(t *testing.T) {
	resourceNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		ShipwrightBuild: "shipwright-build",
		TargetNamespace: "tekton-pipelines",
	}

	clients := client.Setup(t, resourceNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, resourceNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownShipwrightBuild(clients, resourceNames.ShipwrightBuild) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, resourceNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, resourceNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, resourceNames.TektonPipeline)
	defer utils.TearDownShipwrightBuild(clients, resourceNames.ShipwrightBuild)

	resources.EnsureNoTektonConfigInstance(t, clients, resourceNames)

	// Create a TektonPipeline, as ShipwrightBuild depends on it being ready.
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), resourceNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", resourceNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, resourceNames)
	})

	// Create a ShipwrightBuild
	if _, err := resources.EnsureShipwrightBuildExists(clients.ShipwrightBuild(), resourceNames); err != nil {
		t.Fatalf("ShipwrightBuild %q failed to create: %v", resourceNames.ShipwrightBuild, err)
	}

	// Test if ShipwrightBuild can reach the READY status
	t.Run("create-shipwrightbuild", func(t *testing.T) {
		resources.AssertShipwrightBuildReadyStatus(t, clients, resourceNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-shipwrightbuild-deployments", func(t *testing.T) {
		resources.AssertShipwrightBuildReadyStatus(t, clients, resourceNames)
		resources.DeleteAndVerifyDeployments(t, clients, resourceNames.TargetNamespace, utils.ShipwrightBuildDeploymentLabel)
		resources.AssertShipwrightBuildReadyStatus(t, clients, resourceNames)
	})

	// Test if TektonInstallerSets are created.
	t.Run("verify-shipwrightbuild-installersets", func(t *testing.T) {
		resources.AssertShipwrightBuildInstallerSets(t, clients)
	})

	// Delete the ShipwrightBuild CR instance to see if all resources will be removed
	t.Run("delete-shipwrightbuild", func(t *testing.T) {
		resources.AssertShipwrightBuildReadyStatus(t, clients, resourceNames)
		resources.DeleteShipwrightBuild(t, clients, resourceNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, resourceNames)
		resources.TektonPipelineCRDelete(t, clients, resourceNames)
	})
}
