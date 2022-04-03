//go:build e2e
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

package openshift

import (
	"os"
	"testing"

	"github.com/tektoncd/operator/test/utils"

	"github.com/tektoncd/operator/test/client"

	"github.com/tektoncd/operator/test/resources"
)

// TestTektonAddonsDeployment verifies the TektonAddons creation, deployment recreation, and TektonAddons deletion.
func TestTektonAddonsDeployment(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonTrigger:   "trigger",
		TektonAddon:     "addon",
		Namespace:       "",
		TargetNamespace: "tekton-pipelines",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)

	utils.CleanupOnInterrupt(func() { utils.TearDownTrigger(clients, crNames.TektonTrigger) })
	defer utils.TearDownTrigger(clients, crNames.TektonTrigger)

	utils.CleanupOnInterrupt(func() { utils.TearDownAddon(clients, crNames.TektonAddon) })
	defer utils.TearDownAddon(clients, crNames.TektonAddon)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create a TektonTrigger
	if _, err := resources.EnsureTektonTriggerExists(clients.TektonTrigger(), crNames); err != nil {
		t.Fatalf("TektonTrigger %q failed to create: %v", crNames.TektonTrigger, err)
	}

	// Test if TektonTrigger can reach the READY status
	t.Run("create-trigger", func(t *testing.T) {
		resources.AssertTektonTriggerCRReadyStatus(t, clients, crNames)
	})

	// Create a TektonAddon
	if _, err := resources.EnsureTektonAddonExists(clients.TektonAddon(), crNames); err != nil {
		t.Fatalf("TektonAddon %q failed to create: %v", crNames.TektonAddon, err)
	}

	// Test if TektonAddon can reach the READY status
	t.Run("create-addon", func(t *testing.T) {
		resources.AssertTektonAddonCRReadyStatus(t, clients, crNames)
	})

	// Test if TektonInstallerSets are created.
	t.Run("verify-tektoninstallersets", func(t *testing.T) {
		resources.AssertTektonInstallerSets(t, clients)
	})

	// Delete the TektonAddon CR instance to see if all resources will be removed
	t.Run("delete-addon", func(t *testing.T) {
		resources.AssertTektonAddonCRReadyStatus(t, clients, crNames)
		resources.TektonAddonCRDelete(t, clients, crNames)
	})

	// Delete the TektonTrigger CR instance to see if all resources will be removed
	t.Run("delete-trigger", func(t *testing.T) {
		resources.AssertTektonTriggerCRReadyStatus(t, clients, crNames)
		resources.TektonTriggerCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})

}
