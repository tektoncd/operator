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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// HubClusterResultConfigTestSuite tests the hub cluster result configuration
// When multicluster is enabled and role is Hub, the TektonResult CR should have
// zero replicas for watcher and retention-policy-agent deployments
type HubClusterResultConfigTestSuite struct {
	resourceNames utils.ResourceNames
	suite.Suite
	clients  *utils.Clients
	interval time.Duration
	timeout  time.Duration
	logger   *zap.SugaredLogger
}

func TestHubClusterResultConfigTestSuite(t *testing.T) {
	ts := NewHubClusterResultConfigTestSuite(t)
	suite.Run(t, ts)
}

func NewHubClusterResultConfigTestSuite(t *testing.T) *HubClusterResultConfigTestSuite {
	ts := HubClusterResultConfigTestSuite{
		resourceNames: utils.GetResourceNames(),
		interval:      5 * time.Second,
		timeout:       5 * time.Minute,
		logger:        utils.Logger(),
	}

	ts.clients = client.Setup(t, ts.resourceNames.TargetNamespace)
	ts.SetT(t)

	return &ts
}

func (s *HubClusterResultConfigTestSuite) SetupSuite() {
	resources.PrintClusterInformation(s.logger, s.resourceNames)

	// Skip entire suite if TektonConfig doesn't exist
	_, err := s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	if err != nil {
		s.T().Skipf("TektonConfig not found, skipping hub cluster result config tests: %v", err)
	}
}

func (s *HubClusterResultConfigTestSuite) TearDownTest() {
	t := s.T()
	if t.Failed() {
		s.logger.Infow("test failed, executing debug commands", "testName", t.Name())
		resources.ExecuteDebugCommands(s.logger, s.resourceNames)
	}

	// Reset multicluster config to disabled state after each test (only if TektonConfig exists)
	s.resetMultiClusterConfig()
}

// Test01_HubClusterResultConfig tests that when multicluster is enabled with Hub role,
// the TektonResult CR has zero replicas for watcher and retention-policy-agent
func (s *HubClusterResultConfigTestSuite) Test01_HubClusterResultConfig() {
	t := s.T()

	// Skip if Result is disabled in the config
	tc, err := s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonConfig")

	if tc.Spec.Result.Disabled {
		t.Skip("TektonResult is disabled, skipping hub cluster result config test")
	}

	// Check if TektonResult CR exists
	_, err = s.clients.TektonResult().Get(context.TODO(), v1alpha1.ResultResourceName, metav1.GetOptions{})
	if err != nil {
		t.Skipf("TektonResult CR not found, skipping hub cluster result config test: %v", err)
	}

	// Enable multicluster with Hub role
	s.logger.Debug("enabling multicluster with Hub role")
	tc, err = s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonConfig")

	tc.Spec.Scheduler.MultiClusterDisabled = false
	tc.Spec.Scheduler.MultiClusterRole = v1alpha1.MultiClusterRoleHub

	_, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err, "failed to update TektonConfig with Hub role")

	// Wait for TektonConfig to be ready
	err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, s.interval, s.timeout)
	require.NoError(t, err, "TektonConfig failed to become ready")

	// Verify TektonResult CR has zero replicas for watcher and retention-policy-agent
	s.logger.Debug("verifying TektonResult CR has zero replicas for watcher and retention-policy-agent")
	s.verifyResultDeploymentReplicas("tekton-results-watcher", 0)
	s.verifyResultDeploymentReplicas("tekton-results-retention-policy-agent", 0)
}

// Test02_SpokeClusterResultConfig tests that when multicluster is enabled with Spoke role,
// the TektonResult CR does NOT have zero replicas forced for watcher and retention-policy-agent
func (s *HubClusterResultConfigTestSuite) Test02_SpokeClusterResultConfig() {
	t := s.T()

	// Skip if Result is disabled in the config
	tc, err := s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonConfig")

	if tc.Spec.Result.Disabled {
		t.Skip("TektonResult is disabled, skipping spoke cluster result config test")
	}

	// Check if TektonResult CR exists
	_, err = s.clients.TektonResult().Get(context.TODO(), v1alpha1.ResultResourceName, metav1.GetOptions{})
	if err != nil {
		t.Skipf("TektonResult CR not found, skipping spoke cluster result config test: %v", err)
	}

	// Enable multicluster with Spoke role
	s.logger.Debug("enabling multicluster with Spoke role")
	tc, err = s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonConfig")

	tc.Spec.Scheduler.MultiClusterDisabled = false
	tc.Spec.Scheduler.MultiClusterRole = v1alpha1.MultiClusterRoleSpoke

	_, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err, "failed to update TektonConfig with Spoke role")

	// Wait for TektonConfig to be ready
	err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, s.interval, s.timeout)
	require.NoError(t, err, "TektonConfig failed to become ready")

	// Verify TektonResult CR does NOT have zero replicas forced
	// (replicas should be default or user-specified, not 0)
	s.logger.Debug("verifying TektonResult CR does not have forced zero replicas for Spoke role")
	s.verifyResultDeploymentReplicasNotZero("tekton-results-watcher")
	s.verifyResultDeploymentReplicasNotZero("tekton-results-retention-policy-agent")
}

// Test03_MultiClusterDisabled tests that when multicluster is disabled,
// the TektonResult CR does NOT have zero replicas forced
func (s *HubClusterResultConfigTestSuite) Test03_MultiClusterDisabled() {
	t := s.T()

	// Skip if Result is disabled in the config
	tc, err := s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonConfig")

	if tc.Spec.Result.Disabled {
		t.Skip("TektonResult is disabled, skipping multicluster disabled test")
	}

	// Check if TektonResult CR exists
	_, err = s.clients.TektonResult().Get(context.TODO(), v1alpha1.ResultResourceName, metav1.GetOptions{})
	if err != nil {
		t.Skipf("TektonResult CR not found, skipping multicluster disabled test: %v", err)
	}

	// Ensure multicluster is disabled
	s.logger.Debug("ensuring multicluster is disabled")
	tc, err = s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonConfig")

	tc.Spec.Scheduler.MultiClusterDisabled = true
	tc.Spec.Scheduler.MultiClusterRole = v1alpha1.MultiClusterRoleHub // Even with Hub role, should not apply when disabled

	_, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err, "failed to update TektonConfig with multicluster disabled")

	// Wait for TektonConfig to be ready
	err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, s.interval, s.timeout)
	require.NoError(t, err, "TektonConfig failed to become ready")

	// Verify TektonResult CR does NOT have zero replicas forced
	s.logger.Debug("verifying TektonResult CR does not have forced zero replicas when multicluster is disabled")
	s.verifyResultDeploymentReplicasNotZero("tekton-results-watcher")
	s.verifyResultDeploymentReplicasNotZero("tekton-results-retention-policy-agent")
}

// Helper functions

func (s *HubClusterResultConfigTestSuite) resetMultiClusterConfig() {
	tc, err := s.clients.TektonConfig().Get(context.TODO(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	if err != nil {
		s.logger.Debugw("TektonConfig not found for reset, skipping", "error", err)
		return
	}

	// Reset to default disabled state
	tc.Spec.Scheduler.MultiClusterDisabled = true
	tc.Spec.Scheduler.MultiClusterRole = ""

	_, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	if err != nil {
		s.logger.Warnw("failed to reset multicluster config", "error", err)
		return
	}

	// Wait for config to be ready after reset (don't fail the test if this times out)
	err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, s.interval, 2*time.Minute)
	if err != nil {
		s.logger.Warnw("TektonConfig did not become ready after reset", "error", err)
	}
}

func (s *HubClusterResultConfigTestSuite) verifyResultDeploymentReplicas(deploymentName string, expectedReplicas int32) {
	t := s.T()

	// Wait for TektonResult CR to be updated with the expected replicas
	err := wait.PollUntilContextTimeout(context.TODO(), s.interval, s.timeout, true, func(ctx context.Context) (bool, error) {
		result, err := s.clients.TektonResult().Get(context.TODO(), v1alpha1.ResultResourceName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		deployment, exists := result.Spec.Options.Deployments[deploymentName]
		if !exists {
			s.logger.Debugw("deployment not found in TektonResult CR", "deployment", deploymentName)
			return false, nil
		}

		if deployment.Spec.Replicas == nil {
			s.logger.Debugw("deployment replicas not set", "deployment", deploymentName)
			return false, nil
		}

		if *deployment.Spec.Replicas == expectedReplicas {
			return true, nil
		}

		s.logger.Debugw("waiting for deployment replicas to be updated",
			"deployment", deploymentName,
			"expected", expectedReplicas,
			"actual", *deployment.Spec.Replicas)
		return false, nil
	})

	require.NoError(t, err, "TektonResult CR %s deployment replicas did not reach expected value %d", deploymentName, expectedReplicas)
}

func (s *HubClusterResultConfigTestSuite) verifyResultDeploymentReplicasNotZero(deploymentName string) {
	t := s.T()

	// Get TektonResult CR and verify the deployment replicas are not forced to 0
	result, err := s.clients.TektonResult().Get(context.TODO(), v1alpha1.ResultResourceName, metav1.GetOptions{})
	require.NoError(t, err, "failed to get TektonResult CR")

	deployment, exists := result.Spec.Options.Deployments[deploymentName]
	if !exists {
		// If deployment doesn't exist in Options, that's fine - it means replicas weren't forced
		s.logger.Debugw("deployment not found in TektonResult Options (not forced)", "deployment", deploymentName)
		return
	}

	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
		t.Errorf("expected %s deployment replicas to NOT be forced to 0, but it was", deploymentName)
	}
}
