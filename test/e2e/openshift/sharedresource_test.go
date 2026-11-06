//go:build e2e
// +build e2e

/*
Copyright 2026 The Tekton Authors

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
	"context"
	"os"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestSharedResourceDeployment verifies the SharedResource creation, deployment recreation, and SharedResource deletion.
func TestSharedResource(t *testing.T) {
	resourceNames := utils.ResourceNames{
		TektonConfig:    "config",
		SharedResource:  "shared-resource",
		Namespace:       "",
		TargetNamespace: "openshift-pipelines",
	}

	clients := client.Setup(t, resourceNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownSharedResource(clients, resourceNames.SharedResource) })
	defer utils.TearDownSharedResource(clients, resourceNames.SharedResource)

	resources.EnsureNoTektonConfigInstance(t, clients, resourceNames)

	// Create a SharedResource. Unlike ShipwrightBuild, it has no TektonPipeline dependency.
	if _, err := resources.EnsureSharedResourceExists(clients.SharedResource(), resourceNames); err != nil {
		t.Fatalf("SharedResource %q failed to create: %v", resourceNames.SharedResource, err)
	}

	// Test if SharedResource can reach the READY status
	t.Run("create-sharedresource", func(t *testing.T) {
		resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-sharedresource-deployments", func(t *testing.T) {
		resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)
		resources.DeleteAndVerifyDeployments(t, clients, resourceNames.TargetNamespace, utils.SharedResourceDeploymentLabel)
		resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)
	})

	// Delete the SharedResource CR instance to see if all resources will be removed
	t.Run("delete-sharedresource", func(t *testing.T) {
		resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)
		resources.DeleteSharedResource(t, clients, resourceNames)
	})
}

// TestSharedResourceNetworkPolicy verifies that the SharedResource NetworkPolicies
// are created by default and that toggling spec.networkPolicy.disabled correctly
// removes and restores the policies.
func TestSharedResourceNetworkPolicy(t *testing.T) {
	resourceNames := utils.ResourceNames{
		TektonConfig:    "config",
		SharedResource:  "shared-resource",
		Namespace:       "",
		TargetNamespace: "tekton-pipelines",
	}

	clients := client.Setup(t, resourceNames.TargetNamespace)

	if os.Getenv("TARGET") == "openshift" {
		resourceNames.TargetNamespace = "openshift-pipelines"
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownSharedResource(clients, resourceNames.SharedResource) })
	defer utils.TearDownSharedResource(clients, resourceNames.SharedResource)

	resources.EnsureNoTektonConfigInstance(t, clients, resourceNames)

	// Create a SharedResource. Unlike ShipwrightBuild, it has no TektonPipeline dependency.
	if _, err := resources.EnsureSharedResourceExists(clients.SharedResource(), resourceNames); err != nil {
		t.Fatalf("SharedResource %q failed to create: %v", resourceNames.SharedResource, err)
	}
	resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)

	expectedPolicies := []string{
		"shared-resource-default-deny",
		"shared-resource-csi-driver-node",
		"shared-resource-csi-driver-webhook",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, resourceNames.TargetNamespace, expectedPolicies)
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		sr, err := clients.SharedResource().Get(context.TODO(), resourceNames.SharedResource, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get SharedResource: %v", err)
		}
		sr.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.SharedResource().Update(context.TODO(), sr, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on SharedResource: %v", err)
		}
		resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, resourceNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		sr, err := clients.SharedResource().Get(context.TODO(), resourceNames.SharedResource, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get SharedResource: %v", err)
		}
		sr.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.SharedResource().Update(context.TODO(), sr, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on SharedResource: %v", err)
		}
		resources.AssertSharedResourceReadyStatus(t, clients, resourceNames)
		resources.AssertNetworkPoliciesExist(t, clients, resourceNames.TargetNamespace, expectedPolicies)
	})
}
