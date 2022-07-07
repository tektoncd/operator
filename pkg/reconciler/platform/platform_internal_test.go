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

package platform

import (
	"reflect"
	"testing"

	"knative.dev/pkg/injection"
)

func TestActiveControllers(t *testing.T) {
	tests := []struct {
		description         string
		ctrlNames           []ControllerName
		supportedCtrls      ControllerMap
		expectedActiveCtrls ControllerMap
	}{
		{
			description: "return ControllerMap populated with controllers that should be run when controllerNames are not nil",

			ctrlNames: []ControllerName{ControllerName("ctrl1"), ControllerName("ctrl2")},
			supportedCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
				ControllerName("ctrl3"): injection.NamedControllerConstructor{},
			},
			expectedActiveCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
			},
		},
		{
			description: "return empty ControllerMap when controllerNames are nil",

			ctrlNames: []ControllerName{},
			supportedCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
				ControllerName("ctrl3"): injection.NamedControllerConstructor{},
			},
			expectedActiveCtrls: ControllerMap{},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			fp := SeededFakePlatform(test.ctrlNames, test.supportedCtrls)
			got := activeControllers(fp)
			if !reflect.DeepEqual(got, test.expectedActiveCtrls) {
				t.Errorf("expecte %v, got %v", got, test.expectedActiveCtrls)
			}

		})
	}
}

func TestDisabledControllers(t *testing.T) {
	tests := []struct {
		description           string
		ctrlNames             []ControllerName
		supportedCtrls        ControllerMap
		expectedDisabledCtrls ControllerMap
	}{
		{
			description: "return ControllerMap populated with controllers that should not be run when controllerNames are not nil",

			ctrlNames: []ControllerName{ControllerName("ctrl1")},
			supportedCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
				ControllerName("ctrl3"): injection.NamedControllerConstructor{},
			},
			expectedDisabledCtrls: ControllerMap{
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
				ControllerName("ctrl3"): injection.NamedControllerConstructor{},
			},
		},
		{
			description: "return full supported ControllerMap when controllerNames are nil",

			ctrlNames: []ControllerName{},
			supportedCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
				ControllerName("ctrl3"): injection.NamedControllerConstructor{},
			},
			expectedDisabledCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
				ControllerName("ctrl3"): injection.NamedControllerConstructor{},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			fp := SeededFakePlatform(test.ctrlNames, test.supportedCtrls)
			got := disabledControllers(fp)
			if !reflect.DeepEqual(got, test.expectedDisabledCtrls) {
				t.Errorf("expected %v, got %v", got, test.expectedDisabledCtrls)
			}

		})
	}
}

func TestValidateControllerNames(t *testing.T) {
	tests := []struct {
		description    string
		ctrlNames      []ControllerName
		supportedCtrls ControllerMap

		errExpected bool
		errVal      error
	}{
		{
			description: "return nil when all give controller names are supported (all names are present in the map)",

			ctrlNames: []ControllerName{ControllerName("ctrl1"), ControllerName("ctrl2")},
			supportedCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{},
				ControllerName("ctrl2"): injection.NamedControllerConstructor{},
			},
			errExpected: false,
		},
		{
			description: "return error some of the ControllerName are not supported",
			ctrlNames:   []ControllerName{ControllerName("ctrl1"), ControllerName("ctrlx")},
			supportedCtrls: ControllerMap{
				ControllerName("ctrl1"): injection.NamedControllerConstructor{Name: "ctrl1"},
			},
			errExpected: true,
			errVal:      ErrorControllerNames("ctrlx", []string{"ctrl1"}),
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			fp := SeededFakePlatform(test.ctrlNames, test.supportedCtrls)
			err := validateControllerNames(fp)
			if test.errExpected {
				AssertError(t, err, test.errVal)
			} else {
				AssertNoError(t, err)
			}

		})
	}
}
