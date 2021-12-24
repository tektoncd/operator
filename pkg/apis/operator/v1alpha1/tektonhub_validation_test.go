/*
Copyright 2022 The Tekton Authors

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
)

func Test_ValidateTektonConfig_InvalidHubParam(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Hub: Hub{
				Params: []Param{
					{
						Name:  "invalid-param",
						Value: "val",
					},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid key name \"invalid-param\": spec.hub.params", err.Error())
}

func Test_ValidateTektonConfig_InvalidHubParamValue(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Hub: Hub{
				Params: []Param{
					{
						Name:  "enable-devconsole-integration",
						Value: "test",
					},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.hub.params.enable-devconsole-integration[0]", err.Error())
}
