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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func Test_ValidateTektonAddon_OnDelete(t *testing.T) {

	ta := &TektonAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonAddonSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	err := ta.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonAddon.Validate() on Delete expected no error, but got one, ValidateTektonAddon: %v", err)
	}
}

func Test_ValidateTektonAddon_InvalidParam(t *testing.T) {

	ta := &TektonAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonAddonSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Params: []Param{
				{
					Name:  "foo",
					Value: "test",
				},
			},
		},
	}

	err := ta.Validate(context.TODO())
	assert.Equal(t, "invalid key name \"foo\": spec.params", err.Error())
}

func Test_ValidateTektonAddon_InvalidParamValue(t *testing.T) {

	ta := &TektonAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonAddonSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Params: []Param{
				{
					Name:  "clusterTasks",
					Value: "test",
				},
			},
		},
	}

	err := ta.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.params.clusterTasks[0]", err.Error())
}

func Test_ValidateTektonAddon_ClusterTaskIsFalseAndPipelineTemplateIsTrue(t *testing.T) {

	ta := &TektonAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonAddonSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Params: []Param{
				{
					Name:  "clusterTasks",
					Value: "false",
				},
				{
					Name:  "pipelineTemplates",
					Value: "true",
				},
			},
		},
	}

	err := ta.Validate(context.TODO())
	assert.Equal(t, "pipelineTemplates cannot be true if clusterTask is false: spec.params", err.Error())
}
