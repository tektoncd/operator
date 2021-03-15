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
	"testing"

	"github.com/tektoncd/operator/test/utils"

	"github.com/tektoncd/operator/test/resources"

	"github.com/tektoncd/operator/test/client"
)

// TestTektonAddonsDeployment verifies the TektonAddons creation, deployment recreation, and TektonAddons deletion.
func TestRBACReconciler(t *testing.T) {
	clients := client.Setup(t)

	crNames := utils.ResourceNames{
		Namespace: "operator-test-rbac",
	}
	expectedSAName := "pipeline"

	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.Namespace) })
	defer utils.TearDownNamespace(clients, crNames.Namespace)

	//// Create a Test Namespace
	if _, err := resources.EnsureTestNamespaceExists(clients, crNames.Namespace); err != nil {
		t.Fatalf("failed to create test namespace: %s, %q", crNames.Namespace, err)
	}

	// Test whether the `pipelineSa` is created in a "default" namespace
	t.Run("verify-service-account", func(t *testing.T) {
		resources.AssertServiceAccount(t, clients, crNames.Namespace, expectedSAName)
	})

	// Test whether the roleBindings are created in a "default" namespace
	t.Run("verify-rolebindings", func(t *testing.T) {
		resources.AssertRoleBinding(t, clients, crNames.Namespace, "edit")
		resources.AssertRoleBinding(t, clients, crNames.Namespace, "pipeline-anyuid")
	})
}
