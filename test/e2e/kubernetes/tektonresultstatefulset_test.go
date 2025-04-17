//go:build e2e
// +build e2e

/*
Copyright 2025 The Tekton Authors

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
	"os"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

// TestTektonResultDeployment verifies the TektonResult creation, deployment recreation, and TektonResult deletion.
func TestTektonResultsWatcherStatefulset(t *testing.T) {
	platform := os.Getenv("PLATFORM")
	if platform == "linux/ppc64le" || platform == "linux/s390x" {
		t.Skipf("Tekton Result is not available for %q", platform)
	}

	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonResult:    "result",
		TargetNamespace: "tekton-pipelines",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownResult(clients, crNames.TektonResult) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownResult(clients, crNames.TektonResult)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Before Installing Results, create the required secrets
	t.Run("create-secrets", func(t *testing.T) {
		createSecret(t, clients, crNames.TargetNamespace)
	})

	// Create a TektonResult
	if _, err := resources.EnsureTektonResultWithStatefulsetExists(clients.TektonResult(), crNames); err != nil {
		t.Fatalf("TektonResult %q failed to create: %v", crNames.TektonResult, err)
	}

	// Test if TektonResult can reach the READY status
	t.Run("create-result", func(t *testing.T) {
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
	})

	t.Run("restore-result-watcher-statefulset", func(t *testing.T) {
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyStatefulSet(t, clients, crNames.TargetNamespace, utils.TektonResultsDeploymentLabel)
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonResult CR instance to see if all resources will be removed
	t.Run("delete-result", func(t *testing.T) {
		resources.AssertTektonResultCRReadyStatus(t, clients, crNames)
		resources.TektonResultCRDDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
