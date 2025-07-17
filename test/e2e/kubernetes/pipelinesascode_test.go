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

// TestKubernetesPipelinesAsCode verifies the PipelinesAsCode CR creation,
// additional controller creation and deletion, and PipelinesAsCode deletion
// on a plain Kubernetes cluster.
func TestKubernetesPipelinesAsCode(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:             "config",
		TektonPipeline:           "pipeline",
		OpenShiftPipelinesAsCode: "pipelines-as-code", // reused field, but on K8s this maps to generic PAC
		Namespace:                "",
		TargetNamespace:          "tekton-pipelines",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	// allow TARGET=kubernetes override (parallel with openshift test)
	if os.Getenv("TARGET") == "kubernetes" {
		crNames.TargetNamespace = "tekton-pipelines"
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.OpenShiftPipelinesAsCode) })
	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })

	defer utils.TearDownNamespace(clients, crNames.OpenShiftPipelinesAsCode)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// 1) create TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// 2) create the PipelinesAsCode CR (generic, not openshift)
	if _, err := resources.EnsureOpenShiftPipelinesAsCodeExists(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("PipelinesAsCode %q failed to create: %v", crNames.OpenShiftPipelinesAsCode, err)
	}
	t.Run("create-kubernetes-pipelines-as-code", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	})

	// create pac resources for additional controller

	if err := resources.CreatePACResources(clients.KubeClient, "tekton-pipelines", "additional-test-configmap", "additional-test-secret"); err != nil {
		t.Fatalf("failed to create resources for additional pipelines-as-code controller in %q: %v", crNames.OpenShiftPipelinesAsCode, err)
	}
	// 3) add additional controller
	if _, err := resources.CreateAdditionalPipelinesAsCodeController(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("failed to create additional pipelines-as-code controller in %q: %v", crNames.OpenShiftPipelinesAsCode, err)
	}
	t.Run("create-additional-pipelines-as-code-controller", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	})

	// wait for the new Deployment to appear
	if err := resources.WaitForDeploymentReady(clients.KubeClient, deploymentName, crNames.TargetNamespace, interval, timeout); err != nil {
		t.Fatalf("additional PAC deployment %q not ready: %v", deploymentName, err)
	}
	if err := resources.WaitForDeploymentAvailable(clients.KubeClient, deploymentName, crNames.TargetNamespace, interval, timeout); err != nil {
		t.Fatalf("additional PAC deployment %q not available: %v", deploymentName, err)
	}

	// 4) remove additional controller
	if _, err := resources.RemoveAdditionalPipelinesAsCodeController(clients.OpenShiftPipelinesAsCode(), crNames); err != nil {
		t.Fatalf("failed to remove additional pipelines-as-code controller in %q: %v", crNames.OpenShiftPipelinesAsCode, err)
	}
	t.Run("remove-additional-controller-pipelines-as-code", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
	})
	if err := resources.WaitForDeploymentDeletion(clients.KubeClient, deploymentName, crNames.TargetNamespace, interval, timeout); err != nil {
		t.Fatalf("additional PAC deployment %q still exists: %v", deploymentName, err)
	}

	// 5) delete the PipelinesAsCode CR
	t.Run("delete-kubernetes-pipelines-as-code", func(t *testing.T) {
		resources.AssertOpenShiftPipelinesAsCodeCRReadyStatus(t, clients, crNames)
		resources.OpenShiftPipelinesAsCodeCRDelete(t, clients, crNames)
	})

	// 6) delete the TektonPipeline CR
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
