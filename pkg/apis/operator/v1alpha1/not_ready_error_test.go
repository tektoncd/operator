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

package v1alpha1

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestNotReadyError(t *testing.T) {
	tests := []struct {
		name       string
		conditions apis.Conditions
		wantMsg    string
	}{
		{
			name:    "no conditions yet",
			wantMsg: RECONCILE_AGAIN_ERR.Error(),
		},
		{
			name: "still installing",
			conditions: apis.Conditions{
				{Type: apis.ConditionReady, Status: corev1.ConditionUnknown},
			},
			wantMsg: RECONCILE_AGAIN_ERR.Error(),
		},
		{
			name: "failed sub condition wins over the ready message",
			conditions: apis.Conditions{
				{Type: InstallerSetReady, Status: corev1.ConditionFalse, Message: "Install failed with message: spec.maxReplicas: Invalid value: 1"},
				{Type: apis.ConditionReady, Status: corev1.ConditionFalse, Message: "TektonInstallerSet not ready"},
			},
			wantMsg: "reconcile again and proceed: Install failed with message: spec.maxReplicas: Invalid value: 1",
		},
		{
			name: "only the ready condition has a message",
			conditions: apis.Conditions{
				{Type: InstallerSetReady, Status: corev1.ConditionTrue},
				{Type: apis.ConditionReady, Status: corev1.ConditionFalse, Message: "Upgrade Pending"},
			},
			wantMsg: "reconcile again and proceed: Upgrade Pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &TektonPipelineStatus{Status: duckv1.Status{Conditions: duckv1.Conditions(tt.conditions)}}
			err := NotReadyError(status)
			if !errors.Is(err, RECONCILE_AGAIN_ERR) {
				t.Fatalf("errors.Is(%v, RECONCILE_AGAIN_ERR) = false", err)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestNotReadyErrorWithoutStatus(t *testing.T) {
	if err := NotReadyError(nil); err != RECONCILE_AGAIN_ERR {
		t.Errorf("NotReadyError(nil) = %v, want RECONCILE_AGAIN_ERR", err)
	}
}
