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

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func Test_ValidateTektonTrigger_MissingTargetNamespace(t *testing.T) {

	tr := &TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trigger",
			Namespace: "namespace",
		},
		Spec: TektonTriggerSpec{},
	}

	err := tr.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonTrigger_APIField(t *testing.T) {

	tp := &TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trigger",
			Namespace: "namespace",
		},
		Spec: TektonTriggerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Trigger: Trigger{
				TriggersProperties: TriggersProperties{
					EnableApiFields: "prod",
				},
			},
		},
	}

	err := tp.Validate(context.TODO())
	assert.Equal(t, "invalid value: prod: spec.enable-api-fields", err.Error())
}

func Test_ValidateTektonTrigger_OnDelete(t *testing.T) {

	td := &TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonTriggerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	err := td.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonTrigger.Validate() on Delete expected no error, but got one, ValidateTektonTrigger: %v", err)
	}
}
