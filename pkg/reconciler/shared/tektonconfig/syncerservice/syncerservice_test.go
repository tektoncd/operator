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

package syncerservice

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestIsSyncerServiceEnabled(t *testing.T) {
	tests := []struct {
		name      string
		scheduler *v1alpha1.Scheduler
		expected  bool
	}{
		{
			name:      "nil scheduler returns false",
			scheduler: nil,
			expected:  false,
		},
		{
			name: "multi-cluster enabled with Hub role returns true",
			scheduler: &v1alpha1.Scheduler{
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: false,
					MultiClusterRole:     v1alpha1.MultiClusterRoleHub,
				},
			},
			expected: true,
		},
		{
			name: "multi-cluster enabled with Spoke role returns false",
			scheduler: &v1alpha1.Scheduler{
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: false,
					MultiClusterRole:     v1alpha1.MultiClusterRoleSpoke,
				},
			},
			expected: false,
		},
		{
			name: "multi-cluster disabled with Hub role returns false",
			scheduler: &v1alpha1.Scheduler{
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: true,
					MultiClusterRole:     v1alpha1.MultiClusterRoleHub,
				},
			},
			expected: false,
		},
		{
			name: "multi-cluster disabled with Spoke role returns false",
			scheduler: &v1alpha1.Scheduler{
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: true,
					MultiClusterRole:     v1alpha1.MultiClusterRoleSpoke,
				},
			},
			expected: false,
		},
		{
			name: "multi-cluster enabled with empty role returns false",
			scheduler: &v1alpha1.Scheduler{
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: false,
					MultiClusterRole:     "",
				},
			},
			expected: false,
		},
		{
			name: "multi-cluster disabled with empty role returns false",
			scheduler: &v1alpha1.Scheduler{
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: true,
					MultiClusterRole:     "",
				},
			},
			expected: false,
		},
		{
			name:      "empty scheduler (zero values) returns false",
			scheduler: &v1alpha1.Scheduler{},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSyncerServiceEnabled(tt.scheduler)
			if result != tt.expected {
				t.Errorf("IsSyncerServiceEnabled() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsSyncerServiceReady(t *testing.T) {
	tests := []struct {
		name          string
		syncerService *v1alpha1.SyncerService
		expectedReady bool
		expectedErr   error
	}{
		{
			name: "syncer service is ready",
			syncerService: &v1alpha1.SyncerService{
				Status: v1alpha1.SyncerServiceStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{
								Type:   apis.ConditionReady,
								Status: "True",
							},
						},
					},
				},
			},
			expectedReady: true,
			expectedErr:   nil,
		},
		{
			name: "syncer service is not ready",
			syncerService: &v1alpha1.SyncerService{
				Status: v1alpha1.SyncerServiceStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{
								Type:    apis.ConditionReady,
								Status:  "False",
								Message: "waiting for dependencies",
							},
						},
					},
				},
			},
			expectedReady: false,
			expectedErr:   nil,
		},
		{
			name: "syncer service has upgrade pending",
			syncerService: &v1alpha1.SyncerService{
				Status: v1alpha1.SyncerServiceStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{
								Type:    apis.ConditionReady,
								Status:  "False",
								Message: v1alpha1.UpgradePending,
							},
						},
					},
				},
			},
			expectedReady: false,
			expectedErr:   v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR,
		},
		{
			name: "nil status conditions",
			syncerService: &v1alpha1.SyncerService{
				Status: v1alpha1.SyncerServiceStatus{},
			},
			expectedReady: false,
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready, err := isSyncerServiceReady(tt.syncerService)

			if ready != tt.expectedReady {
				t.Errorf("isSyncerServiceReady() ready = %v, expected %v", ready, tt.expectedReady)
			}
			if tt.expectedErr != nil {
				if err != tt.expectedErr {
					t.Errorf("isSyncerServiceReady() error = %v, expected %v", err, tt.expectedErr)
				}
			} else if err != nil {
				t.Errorf("isSyncerServiceReady() unexpected error = %v", err)
			}
		})
	}
}

func TestGetSyncerServiceCR(t *testing.T) {
	operatorVersion := "1.0.0"
	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
			UID:  "test-uid",
		},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Config: v1alpha1.Config{
				NodeSelector: map[string]string{
					"node-type": "worker",
				},
			},
		},
	}

	syncerService := GetSyncerServiceCR(config, operatorVersion)

	// Verify name
	if syncerService.Name != v1alpha1.SyncerServiceResourceName {
		t.Errorf("Expected name %s, got %s", v1alpha1.SyncerServiceResourceName, syncerService.Name)
	}

	// Verify owner reference
	if len(syncerService.OwnerReferences) != 1 {
		t.Fatalf("Expected 1 owner reference, got %d", len(syncerService.OwnerReferences))
	}
	if syncerService.OwnerReferences[0].Name != config.Name {
		t.Errorf("Expected owner reference name %s, got %s", config.Name, syncerService.OwnerReferences[0].Name)
	}
	if syncerService.OwnerReferences[0].UID != config.UID {
		t.Errorf("Expected owner reference UID %s, got %s", config.UID, syncerService.OwnerReferences[0].UID)
	}

	// Verify labels
	if syncerService.Labels[v1alpha1.ReleaseVersionKey] != operatorVersion {
		t.Errorf("Expected label %s=%s, got %s", v1alpha1.ReleaseVersionKey, operatorVersion, syncerService.Labels[v1alpha1.ReleaseVersionKey])
	}

	// Verify spec
	if syncerService.Spec.TargetNamespace != "tekton-pipelines" {
		t.Errorf("Expected TargetNamespace tekton-pipelines, got %s", syncerService.Spec.TargetNamespace)
	}

	// Verify config is copied
	if syncerService.Spec.Config.NodeSelector["node-type"] != "worker" {
		t.Errorf("Expected Config.NodeSelector[node-type]=worker, got %v", syncerService.Spec.Config.NodeSelector)
	}
}
