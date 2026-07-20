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
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestMAGAdoptionViaTektonConfig tests the adoption scenario where a standalone
// ManualApprovalGate CR (without ownerReferences) is adopted by TektonConfig.
// This simulates an upgrade from a version where MAG was a standalone component
// to a version where it's managed by TektonConfig.
func TestMAGAdoptionViaTektonConfig(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:       "config",
		ManualApprovalGate: "manual-approval-gate",
		TargetNamespace:    "tekton-pipelines",
		Namespace:          "tekton-operator",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
		crNames.Namespace = "openshift-operators"
	}

	platform := os.Getenv("PLATFORM")
	if platform == "linux/ppc64le" || platform == "linux/s390x" {
		t.Skipf("ManualApprovalGate is not available for %q", platform)
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	// Setup cleanup handlers
	utils.CleanupOnInterrupt(func() { utils.TearDownConfig(clients, crNames.TektonConfig) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownConfig(clients, crNames.TektonConfig)

	// Ensure no existing TektonConfig or ManualApprovalGate instances
	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	t.Run("adopt-standalone-mag-into-tektonconfig", func(t *testing.T) {
		// Step 1: Create a standalone MAG CR (no ownerRef, simulating pre-upgrade state)
		mag := &v1alpha1.ManualApprovalGate{
			ObjectMeta: metav1.ObjectMeta{
				Name: crNames.ManualApprovalGate,
				// Intentionally no OwnerReferences - simulates pre-upgrade state
			},
			Spec: v1alpha1.ManualApprovalGateSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: crNames.TargetNamespace,
				},
			},
		}

		createdMAG, err := clients.ManualApprovalGate().Create(context.TODO(), mag, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create standalone ManualApprovalGate CR")

		// Verify MAG was created without ownerReferences
		assert.Empty(t, createdMAG.OwnerReferences, "Standalone MAG should not have ownerReferences")

		// Step 2: Create TektonConfig CR - this will trigger the adoption logic
		_, err = resources.EnsureTektonConfigExists(clients.KubeClientSet, clients.TektonConfig(), crNames)
		require.NoError(t, err, "Failed to create TektonConfig CR")

		// Step 3: Wait for TektonConfig to reach Ready status
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)

		// Step 4: Assert TektonConfig.Spec.ManualApproval.Disabled == false (adoption happened)
		tc, err := clients.TektonConfig().Get(context.TODO(), crNames.TektonConfig, metav1.GetOptions{})
		require.NoError(t, err, "Failed to get TektonConfig CR")

		assert.NotNil(t, tc.Spec.ManualApproval.Disabled, "ManualApproval.Disabled should not be nil after adoption")
		assert.False(t, *tc.Spec.ManualApproval.Disabled, "ManualApprovalGate should be enabled in TektonConfig after adoption")

		// Step 5: Assert ManualApprovalGate.OwnerReferences contains TektonConfig
		adoptedMAG, err := clients.ManualApprovalGate().Get(context.TODO(), crNames.ManualApprovalGate, metav1.GetOptions{})
		require.NoError(t, err, "Failed to get ManualApprovalGate CR after adoption")

		assert.NotEmpty(t, adoptedMAG.OwnerReferences, "ManualApprovalGate should have ownerReferences after adoption")

		// Verify the ownerReference points to TektonConfig
		foundOwnerRef := false
		for _, ownerRef := range adoptedMAG.OwnerReferences {
			if ownerRef.Kind == "TektonConfig" && ownerRef.Name == crNames.TektonConfig {
				foundOwnerRef = true
				assert.True(t, *ownerRef.Controller, "TektonConfig should be the controller")
				break
			}
		}
		assert.True(t, foundOwnerRef, "ManualApprovalGate should have TektonConfig as ownerReference")

		// Step 6: Assert MAG reaches Ready status
		resources.AssertManualApprovalGateCRReadyStatus(t, clients, crNames)
	})
}
