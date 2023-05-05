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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func Test_ValidateTektonPipeline_MissingTargetNamespace(t *testing.T) {

	tp := &TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline",
			Namespace: "namespace",
		},
		Spec: TektonPipelineSpec{},
	}

	err := tp.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonPipeline_APIField(t *testing.T) {

	tp := &TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline",
			Namespace: "namespace",
		},
		Spec: TektonPipelineSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tests := []struct {
		name     string
		apiField string
		err      string
	}{
		{name: "api-alpha", apiField: config.AlphaAPIFields, err: ""},
		{name: "api-beta", apiField: config.AlphaAPIFields, err: ""},
		{name: "api-stable", apiField: config.AlphaAPIFields, err: ""},
		{name: "api-invalid", apiField: "prod", err: "invalid value: prod: spec.enable-api-fields"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tp.Spec.Pipeline.EnableApiFields = test.apiField
			errs := tp.Validate(context.TODO())
			assert.Equal(t, test.err, errs.Error())
		})
	}

}

func Test_ValidateTektonPipeline_OnDelete(t *testing.T) {

	td := &TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonPipelineSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	err := td.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonPipeline.Validate() on Delete expected no error, but got one, ValidateTektonPipeline: %v", err)
	}
}
