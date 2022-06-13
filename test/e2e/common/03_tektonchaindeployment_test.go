//go:build e2e
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

// TestTektonChainDeployment verifies the TektonChain creation, deployment recreation, and TektonChain deletion.
func TestTektonChainDeployment(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonChain:     "chain",
		TargetNamespace: "tekton-pipelines",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}
	platform := os.Getenv("PLATFORM")
	if platform == "linux/ppc64le" || platform == "linux/s390x" {
		t.Skipf("Tekton chain is not available for %q", platform)
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownChain(clients, crNames.TektonChain) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownChain(clients, crNames.TektonChain)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create a TektonChain
	if _, err := resources.EnsureTektonChainExists(clients.TektonChains(), crNames); err != nil {
		t.Fatalf("TektonChain %q failed to create: %v", crNames.TektonChain, err)
	}

	// Test if TektonChain can reach the READY status
	t.Run("create-chain", func(t *testing.T) {
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-chain-deployments", func(t *testing.T) {
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyDeployments(t, clients, crNames.TargetNamespace, utils.TektonChainDeploymentLabel)
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonChain CR instance to see if all resources will be removed
	t.Run("delete-chain", func(t *testing.T) {
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
		resources.TektonChainCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
