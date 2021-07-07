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

func Test_ValidateTektonConfig_OnDelete(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
		},
	}

	err := tc.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonConfig.Validate() on Delete expected no error, but got one, ValidateTektonConfig: %v", err)
	}
}

func Test_ValidateTektonConfig_MissingTargetNamespace(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonConfig_InvalidProfile(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "test",
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.profile", err.Error())
}

func Test_ValidateTektonConfig_InvalidPruningResource(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pruner: Prune{
				Resources: []string{"task"},
				Schedule:  "",
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: task: spec.pruner.resources[0]\nmissing field(s): spec.pruner.schedule", err.Error())
}

func Test_ValidateTektonConfig_MissingSchedule(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pruner: Prune{
				Resources: []string{"taskrun"},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.pruner.schedule", err.Error())
}

func Test_ValidateTektonConfig_InvalidAddonParam(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Addon: Addon{
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
	assert.Equal(t, "invalid key name \"invalid-param\": spec.addon.params", err.Error())
}

func Test_ValidateTektonConfig_InvalidAddonParamValue(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Addon: Addon{
				Params: []Param{
					{
						Name:  "clusterTasks",
						Value: "test",
					},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.addon.params.clusterTasks[0]", err.Error())
}
