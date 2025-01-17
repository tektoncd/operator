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
)

func Test_AddonSetDefaults(t *testing.T) {
	tests := []struct {
		name           string
		initialParams  []Param
		expectedParams map[string]string
	}{
		{
			name: "Default Params with Values",
			initialParams: []Param{
				{Name: PipelineTemplatesParam, Value: "true"},
			},
			expectedParams: map[string]string{
				PipelineTemplatesParam: "true",
			},
		},
		{
			name: "Resolver Task is False",
			initialParams: []Param{
				{Name: ResolverTasks, Value: "false"},
			},
			expectedParams: map[string]string{
				ResolverTasks: "false",
			},
		},
		{
			name: "Resolver Step Actions",
			initialParams: []Param{
				{Name: ResolverStepActions, Value: "false"},
			},
			expectedParams: map[string]string{
				ResolverStepActions: "false",
			},
		},
		{
			name: "Community Resolver Tasks",
			initialParams: []Param{
				{Name: CommunityResolverTasks, Value: "false"},
			},
			expectedParams: map[string]string{
				CommunityResolverTasks: "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := &TektonAddon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: TektonAddonSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "namespace",
					},
					Addon: Addon{
						Params: tt.initialParams,
					},
				},
			}

			ta.SetDefaults(context.TODO())
			checkAddonParams(t, ta.Spec.Addon.Params, tt.expectedParams)
		})
	}
}

func checkAddonParams(t *testing.T, actualParams []Param, expectedParams map[string]string) {
	t.Helper()

	if len(actualParams) != len(AddonParams) {
		t.Fatalf("Expected %d addon params, got %d", len(AddonParams), len(actualParams))
	}

	paramsMap := ParseParams(actualParams)

	for key, expectedValue := range expectedParams {
		value, exists := paramsMap[key]
		assert.Equal(t, true, exists, "Param %q is missing in Spec.Addon.Params", key)
		assert.Equal(t, expectedValue, value, "Param %q has incorrect value", key)
	}
}
