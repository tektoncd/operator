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

package openshift

import (
	"os"
	"testing"
	"time"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

const (
	interval       = 5 * time.Second
	timeout        = 5 * time.Minute
	deploymentName = "additional-test-pac-controller"
)

// TestOpenshiftPipelinesAsCode verifies the TestOpenshiftPipelinesAsCode creation, additional controller creation and
// deletion, and TestOpenshiftPipelinesAsCode deletion.
func TestOpenshiftPipelinesAsCode(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:             "config",
		TektonPipeline:           "pipeline",
		OpenShiftPipelinesAsCode: "pipelines-as-code",
		Namespace:                "",
		TargetNamespace:          "openshift-pipelines",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.OpenShiftPipelinesAsCode) })
	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })

	defer utils.TearDownNamespace(clients, crNames.OpenShiftPipelinesAsCode)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create the OpenShift Pipelines As Code
	if _, err := resources.EnsureOpenShiftPipelinesAsCodeExists(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("OpenShiftPipelinesAsCode %q failed to create: %v", crNames.OpenShiftPipelinesAsCode, err)
	}

	// Test if OpenShiftPipelinesAsCode can reach the READY status
	t.Run("create-openshift-pipelines-as-code", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	})

	// Create the additional Pipelines As Controller in the OpenShiftPipelinesAsCode CR
	if _, err := resources.CreateAdditionalPipelinesAsCodeController(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("failed to create additional pipelines as code controller in %q: %v", crNames.OpenShiftPipelinesAsCode, err)
	}

	// Test if OpenShiftPipelinesAsCode can reach the READY status after deploying additional controller
	t.Run("create-additional-pipelines-as-code-controller", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	})

	// Wait for additional pipelines as code controller deployment gets ready
	if err := resources.WaitForDeploymentReady(clients.KubeClient, deploymentName, crNames.TargetNamespace, interval, timeout); err != nil {
		t.Fatalf("failed to check ready status of additional pipelines as code deployment with name %q: %v", deploymentName, err)
	}

	// If additional Pipelines As Code deployment is available or not
	if err := resources.WaitForDeploymentAvailable(clients.KubeClient, deploymentName, crNames.TargetNamespace, interval, timeout); err != nil {
		t.Fatalf("failed to check if additional pipelines as code deployment %q is available: %v", deploymentName, err)
	}

	// Remove the additional Pipelines As Controller from the OpenShiftPipelinesAsCode CR
	if _, err := resources.RemoveAdditionalPipelinesAsCodeController(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("failed to remove additional pipelines as code controller in %q: %v", crNames.OpenShiftPipelinesAsCode, err)
	}

	// Test if OpenShiftPipelinesAsCode can reach the READY status after removing additional controller
	t.Run("remove-additional-controller-pipelines-as-code", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	})

	// Wait for additional pipelines as code controller deployment gets deleted
	if err := resources.WaitForDeploymentDeletion(clients.KubeClient, deploymentName, crNames.TargetNamespace, interval, timeout); err != nil {
		t.Fatalf("failed to check if additional pipelines as code deployment %q is deleted: %v", deploymentName, err)
	}

	// Delete the OpenShiftPipelinesAsCode CR instance
	t.Run("delete-openshift-pipelines-as-code", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
		resources.OpenShiftPipelinesAsCodeCRDelete(t, clients, crNames)
	})

	// Delete the TektonPipeline CR instance
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
