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
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetDefaultsManualApprovalGate(t *testing.T) {
	mag := &ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name: ManualApprovalGates,
		},
		Spec: ManualApprovalGateSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}

	mag.SetDefaults(context.TODO())

	if mag.Spec.ManualApproval.Disabled == nil {
		t.Error("expected Disabled to be set, got nil")
	}
	if !*mag.Spec.ManualApproval.Disabled {
		t.Error("expected Disabled to default to true, got false")
	}
}

func TestSetDefaultsManualApprovalGate_DisabledAlreadySet(t *testing.T) {
	disabled := false
	mag := &ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name: ManualApprovalGates,
		},
		Spec: ManualApprovalGateSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			ManualApproval: ManualApproval{
				Disabled: &disabled,
			},
		},
	}

	mag.SetDefaults(context.TODO())

	if mag.Spec.ManualApproval.Disabled == nil {
		t.Error("expected Disabled to remain set, got nil")
	}
	if *mag.Spec.ManualApproval.Disabled {
		t.Error("expected Disabled to remain false, got true")
	}
}

func TestManualApprovalIsDisabled(t *testing.T) {
	tests := []struct {
		name     string
		disabled *bool
		want     bool
	}{
		{
			name:     "nil defaults to disabled",
			disabled: nil,
			want:     true,
		},
		{
			name:     "explicitly disabled",
			disabled: boolPtr(true),
			want:     true,
		},
		{
			name:     "explicitly enabled",
			disabled: boolPtr(false),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ManualApproval{Disabled: tt.disabled}
			if got := m.IsDisabled(); got != tt.want {
				t.Errorf("IsDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
