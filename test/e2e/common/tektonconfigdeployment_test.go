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
	"os"
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
		TektonConfig: common.ConfigResourceName,
		Namespace:    "tekton-operator",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.Namespace = "openshift-operators"
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

	if os.Getenv("TARGET") == "openshift" {
		runRbacTest(t, clients)
	}

	// Delete the TektonConfig CR instance to see if all resources will be removed
	t.Run("delete-config", func(t *testing.T) {
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
		resources.TektonConfigCRDelete(t, clients, crNames)
	})
}

func runRbacTest(t *testing.T, clients *utils.Clients) {

	// Test whether the supporting rbac resources are created for existing namespace and
	// newly created namespace

	existingNamespace := "default"
	testNamespace := "operator-test-rbac"

	// Create a Test Namespace
	if _, err := resources.EnsureTestNamespaceExists(clients, testNamespace); err != nil {
		t.Fatalf("failed to create test namespace: %s, %q", testNamespace, err)
	}

	clusterRoleName := "pipelines-scc-clusterrole"

	t.Run("verify-clusterrole", func(t *testing.T) {
		resources.AssertClusterRole(t, clients, clusterRoleName)
	})

	expectedSAName := "pipeline"

	// Test whether the `pipelineSa` is created in a "default" namespace
	t.Run("verify-service-account", func(t *testing.T) {
		resources.AssertServiceAccount(t, clients, existingNamespace, expectedSAName)
		resources.AssertServiceAccount(t, clients, testNamespace, expectedSAName)
	})

	serviceCABundleConfigMap := "config-service-cabundle"
	trustedCABundleConfigMap := "config-trusted-cabundle"

	// Test whether the configMaps are created
	t.Run("verify-configmaps", func(t *testing.T) {
		resources.AssertConfigMap(t, clients, existingNamespace, serviceCABundleConfigMap)
		resources.AssertConfigMap(t, clients, testNamespace, trustedCABundleConfigMap)
		resources.AssertConfigMap(t, clients, existingNamespace, serviceCABundleConfigMap)
		resources.AssertConfigMap(t, clients, testNamespace, trustedCABundleConfigMap)
	})

	pipelinesSCCRoleBinding := "pipelines-scc-rolebinding"
	editRoleBinding := "edit"

	// Test whether the roleBindings are created
	t.Run("verify-rolebindings", func(t *testing.T) {
		resources.AssertRoleBinding(t, clients, existingNamespace, pipelinesSCCRoleBinding)
		resources.AssertRoleBinding(t, clients, testNamespace, pipelinesSCCRoleBinding)
		resources.AssertRoleBinding(t, clients, existingNamespace, editRoleBinding)
		resources.AssertRoleBinding(t, clients, testNamespace, editRoleBinding)
	})

}
