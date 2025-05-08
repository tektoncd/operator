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
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

func TestTektonPruner(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownTektonPruner(clients, crNames.TektonPruner) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownTektonPruner(clients, crNames.TektonPruner)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline CR
	t.Run("setting-up-tekton-pipeline-CR", func(t *testing.T) {
		if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
			t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
		}
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	t.Run("create-tekton-pruner-CR", func(t *testing.T) {
		if _, err := resources.EnsureTektonPrunerExists(clients.TektonPruner(), crNames); err != nil {
			t.Fatalf("TektonPruner %q failed to create: %v", crNames.TektonPruner, err)
		}
		resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)
	})

	//Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-tekton-pruner-deployments", func(t *testing.T) {
		resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyDeployments(t, clients, crNames.TargetNamespace, utils.TektonPrunerDeploymentLabel)
		resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonPruner CR instance to see if all resource will be removed
	t.Run("delete-tekton-pruner", func(t *testing.T) {
		resources.AssertTektonPrunerCRReadyStatus(t, clients, crNames)
		resources.TektonPrunerCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})

}
