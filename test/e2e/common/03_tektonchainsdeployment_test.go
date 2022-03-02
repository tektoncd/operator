// +build e2e

/*
Copyright 2022 The Tekton Authors

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

// TestTektonChainsDeployment verifies the TektonChains creation, deployment recreation, and TektonChains deletion.
func TestTektonChainsDeployment(t *testing.T) {
	clients := client.Setup(t)

	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonChains:    "chains",
		TargetNamespace: "tekton-pipelines",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)

	utils.CleanupOnInterrupt(func() { utils.TearDownChains(clients, crNames.TektonChains) })
	defer utils.TearDownChains(clients, crNames.TektonChains)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create a TektonChains
	if _, err := resources.EnsureTektonChainsExists(clients.TektonChains(), crNames); err != nil {
		t.Fatalf("TektonChains %q failed to create: %v", crNames.TektonChains, err)
	}

	// Test if TektonChains can reach the READY status
	t.Run("create-chains", func(t *testing.T) {
		resources.AssertTektonChainsCRReadyStatus(t, clients, crNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-chains-deployments", func(t *testing.T) {
		resources.AssertTektonChainsCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyDeployments(t, clients, crNames.TargetNamespace, utils.TektonChainsDeploymentLabel)
		resources.AssertTektonChainsCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonChains CR instance to see if all resources will be removed
	t.Run("delete-chains", func(t *testing.T) {
		resources.AssertTektonChainsCRReadyStatus(t, clients, crNames)
		resources.TektonChainsCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})

}
