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

package platform_test

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/platform"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
)

var (
	ctrlCnstr1 injection.ControllerConstructor = func(ctx context.Context, c configmap.Watcher) *controller.Impl { return nil }
	ctrlCnstr2 injection.ControllerConstructor = func(ctx context.Context, c configmap.Watcher) *controller.Impl { return nil }
)

func TestControllerMap(t *testing.T) {
	tests := []struct {
		description                         string
		cMap                                platform.ControllerMap
		expectedControllerNames             []string
		expectedControllerConstructors      []injection.ControllerConstructor
		expectedNamedControllerConstructors []injection.NamedControllerConstructor
	}{
		{
			description: "returns map keys when map is non-empty",
			cMap: platform.ControllerMap{
				platform.ControllerName("key1"): injection.NamedControllerConstructor{
					Name:                  "name1",
					ControllerConstructor: ctrlCnstr1,
				},
				platform.ControllerName("key2"): injection.NamedControllerConstructor{
					Name:                  "name2",
					ControllerConstructor: ctrlCnstr2,
				},
			},
			expectedControllerNames:        []string{"name1", "name2"},
			expectedControllerConstructors: []injection.ControllerConstructor{ctrlCnstr1, ctrlCnstr2},
			expectedNamedControllerConstructors: []injection.NamedControllerConstructor{
				injection.NamedControllerConstructor{
					Name:                  "name1",
					ControllerConstructor: ctrlCnstr1,
				},
				injection.NamedControllerConstructor{
					Name:                  "name2",
					ControllerConstructor: ctrlCnstr2,
				},
			},
		},
		{
			description:                         "returns empty slice map is empty",
			cMap:                                platform.ControllerMap{},
			expectedControllerNames:             []string{},
			expectedControllerConstructors:      []injection.ControllerConstructor{},
			expectedNamedControllerConstructors: []injection.NamedControllerConstructor{},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// TODO: add more smarter checks involving comparisson of slices
			// as these slices are created by ranging over maps, the order is unpredictable
			gotControllerNames := test.cMap.ControllerNames()
			if gotControllerNames == nil && test.expectedControllerNames != nil {
				t.Errorf("expected: %v, got: %v", test.expectedControllerNames, gotControllerNames)
			} else if len(gotControllerNames) != len(test.expectedControllerNames) {
				t.Errorf("expected: %v, got: %v", test.expectedControllerNames, gotControllerNames)

			}

			gotControllerConstructors := test.cMap.ControllerConstructors()
			if gotControllerConstructors == nil && test.expectedControllerConstructors != nil {
				t.Errorf("expected: %v, got: %v", test.expectedControllerConstructors, gotControllerConstructors)
			} else if len(gotControllerConstructors) != len(test.expectedControllerConstructors) {
				t.Errorf("expected: %v, got: %v", test.expectedControllerConstructors, gotControllerConstructors)

			}

			gotNamedControllerConstructors := test.cMap.NamedControllerConstructors()
			if gotNamedControllerConstructors == nil && test.expectedNamedControllerConstructors != nil {
				t.Errorf("expected: %v, got: %v", test.expectedNamedControllerConstructors, gotNamedControllerConstructors)
			} else if len(gotNamedControllerConstructors) != len(test.expectedNamedControllerConstructors) {
				t.Errorf("expected: %v, got: %v", test.expectedNamedControllerConstructors, gotNamedControllerConstructors)

			}
		})
	}
}
