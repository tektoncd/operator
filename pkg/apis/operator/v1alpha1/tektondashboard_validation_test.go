/*
Copyright 2021 The Tekton Authors

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

	"gotest.tools/assert"
	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ValidateTektonDashboard_MissingTargetNamespace(t *testing.T) {

	td := &TektonDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonDashboardSpec{},
	}

	err := td.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonDashboard_OnDelete(t *testing.T) {

	td := &TektonDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonDashboardSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	err := td.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonDashboard.Validate() on Delete expected no error, but got one, ValidateTektonDashboard: %v", err)
	}
}
