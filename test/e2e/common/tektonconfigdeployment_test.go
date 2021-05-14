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

package common

import (
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

// TestTektonPipelinesDeployment verifies the TektonPipelines creation, deployment recreation, and TektonPipelines deletion.
func TestTektonConfigDeployment(t *testing.T) {
	clients := client.Setup(t)

	crNames := utils.ResourceNames{
		TektonConfig:    common.ConfigResourceName,
		Namespace:       "tekton-operator",
		TargetNamespace: "tekton-pipelines",
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownConfig(clients, crNames.TektonConfig) })
	defer utils.TearDownPipeline(clients, crNames.TektonConfig)

	// Create a TektonConfig
	t.Run("create-config", func(t *testing.T) {
		if _, err := resources.EnsureTektonConfigExists(clients.KubeClientSet, clients.TektonConfig(), crNames); err != nil {
			t.Fatalf("TektonConfig %q failed to create: %v", crNames.TektonConfig, err)
		}
	})

	// Test if TektonConfig can reach the READY status
	t.Run("ensure-config-ready-status", func(t *testing.T) {
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonConfig CR instance to see if all resources will be removed
	t.Run("delete-config", func(t *testing.T) {
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
		resources.TektonConfigCRDelete(t, clients, crNames)
	})
}
