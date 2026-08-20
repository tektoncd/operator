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

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestValidateManualApprovalGate_ValidConfig(t *testing.T) {
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

	err := mag.Validate(context.TODO())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateManualApprovalGate_InvalidResourceName(t *testing.T) {
	mag := &ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-name",
		},
		Spec: ManualApprovalGateSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}

	err := mag.Validate(context.TODO())
	assert.ErrorContains(t, err, "Only one instance of ManualApprovalGate is allowed")
}

func TestValidateManualApprovalGate_MissingTargetNamespace(t *testing.T) {
	mag := &ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name: ManualApprovalGates,
		},
		Spec: ManualApprovalGateSpec{},
	}

	err := mag.Validate(context.TODO())
	assert.ErrorContains(t, err, "missing field(s): spec.targetNamespace")
}

func TestValidateManualApprovalGate_SkipOnDelete(t *testing.T) {
	mag := &ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-name",
		},
	}

	err := mag.Validate(apis.WithinDelete(context.TODO()))
	if err != nil {
		t.Errorf("expected no error on delete, got: %v", err)
	}
}
