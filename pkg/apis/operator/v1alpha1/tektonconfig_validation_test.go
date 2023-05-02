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
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonConfig_InvalidProfile(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "test",
			Pruner:  Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.profile", err.Error())
}

func Test_ValidateTektonConfig_InvalidPruningResource(t *testing.T) {
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
			Pruner: Prune{
				Resources: []string{"task"},
				Schedule:  "",
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "expected exactly one, got neither: spec.pruner.keep, spec.pruner.keep-since\ninvalid value: task: spec.pruner.resources[0]", err.Error())
}

func Test_ValidateTektonConfig_MissingKeepKeepsinceSchedule(t *testing.T) {

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
			Pruner: Prune{
				Resources: []string{"taskrun"},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "expected exactly one, got neither: spec.pruner.keep, spec.pruner.keep-since", err.Error())
}

func Test_ValidateTektonConfig_InvalidAddonParam(t *testing.T) {

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
			Addon: Addon{
				Params: []Param{
					{
						Name:  "invalid-param",
						Value: "val",
					},
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid key name \"invalid-param\": spec.addon.params", err.Error())
}

func Test_ValidateTektonConfig_InvalidAddonParamValue(t *testing.T) {

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
			Addon: Addon{
				Params: []Param{
					{
						Name:  "clusterTasks",
						Value: "test",
					},
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.addon.params.clusterTasks[0]", err.Error())
}

func Test_ValidateTektonConfig_InvalidPipelineProperties(t *testing.T) {

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
			Pipeline: Pipeline{
				PipelineProperties: PipelineProperties{
					EnableApiFields: "test",
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.pipeline.enable-api-fields", err.Error())
}

func Test_ValidateTektonConfig_InvalidTriggerProperties(t *testing.T) {

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
			Trigger: Trigger{
				TriggersProperties: TriggersProperties{
					EnableApiFields: "test",
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.trigger.enable-api-fields", err.Error())
}
