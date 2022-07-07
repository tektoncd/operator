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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var (
	ErrConfigReader = fmt.Errorf("error reading in platform config")
)

func configReaderGen(t testing.TB, sharedMainName string, ctrlNames []ControllerName, configReaderErrorVal error) configReader {
	t.Helper()
	return func(pc *PlatformConfig) error {
		pc.SharedMainName = sharedMainName
		if ctrlNames != nil {
			pc.ControllerNames = make([]ControllerName, len(ctrlNames))
		}
		copy(pc.ControllerNames, ctrlNames)
		return configReaderErrorVal
	}
}

func AssertError(t testing.TB, got, want error) {
	t.Helper()
	if got == nil || got.Error() != want.Error() {
		t.Errorf("expected error: %v but got error: %v", want, got)
	}
}

func AssertNoError(t testing.TB, got error) {
	t.Helper()
	if got != nil {
		t.Errorf("expected not error, but got error: %v", got)
	}
}

func assertPlatformConfig(t testing.TB, got, want PlatformConfig) {
	t.Helper()
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("expected no diff but got: %s", diff)
	}
}

func TestNewConfig(t *testing.T) {

	tests := []struct {
		description          string
		sharedMainName       string
		ctrlNames            []ControllerName
		configReaderErrorVal error
		expected             PlatformConfig
		newConfigError       bool
		newConfigErrorVal    error
	}{
		{
			description:          "returns valid PlatformConfig when shareMainName and ControlerNames are given",
			sharedMainName:       "lifecycle",
			ctrlNames:            []ControllerName{ControllerName("tektonconfig"), ControllerName("tektonpipeline")},
			configReaderErrorVal: nil,
			expected: PlatformConfig{
				Name:            "",
				ControllerNames: []ControllerName{ControllerName("tektonconfig"), ControllerName("tektonpipeline")},
				SharedMainName:  "lifecycle",
			},
			newConfigError:    false,
			newConfigErrorVal: nil,
		},
		{
			description:          "returns valid PlatformConfig when shareMainName is given and ControlerNames is empty slice",
			sharedMainName:       "lifecycle",
			ctrlNames:            []ControllerName{},
			configReaderErrorVal: nil,
			expected: PlatformConfig{
				Name:            "",
				ControllerNames: []ControllerName{},
				SharedMainName:  "lifecycle",
			},
			newConfigError:    false,
			newConfigErrorVal: nil,
		},
		{
			description:          "returns error when configReader returns error",
			sharedMainName:       "lifecycle",
			ctrlNames:            []ControllerName{ControllerName("tektonconfig"), ControllerName("tektonpipeline")},
			configReaderErrorVal: ErrConfigReader,
			expected: PlatformConfig{
				Name:            "",
				ControllerNames: nil,
				SharedMainName:  "",
			},
			newConfigError:    true,
			newConfigErrorVal: ErrConfigReader,
		},
		{
			description:          "returns error sharedMainName is empty",
			sharedMainName:       "",
			ctrlNames:            []ControllerName{ControllerName("tektonconfig"), ControllerName("tektonpipeline")},
			configReaderErrorVal: nil,
			expected: PlatformConfig{
				Name:            "",
				ControllerNames: nil,
				SharedMainName:  "",
			},
			newConfigError:    true,
			newConfigErrorVal: ErrSharedMainNameEmpty,
		},
		{
			description:          "returns error when controllerNames is nil",
			sharedMainName:       "lifecycle",
			ctrlNames:            nil,
			configReaderErrorVal: nil,
			expected: PlatformConfig{
				Name:            "",
				ControllerNames: nil,
				SharedMainName:  "",
			},
			newConfigError:    true,
			newConfigErrorVal: ErrControllerNamesNil,
		},
		{
			description:          "returns combined error when sharedMain name is \"\" and controlelrNames is nil",
			sharedMainName:       "",
			ctrlNames:            nil,
			configReaderErrorVal: nil,
			expected: PlatformConfig{
				Name:            "",
				ControllerNames: nil,
				SharedMainName:  "",
			},
			newConfigError:    true,
			newConfigErrorVal: fmt.Errorf("%s,%s", ErrSharedMainNameEmpty, ErrControllerNamesNil),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			fakeConfigReader := configReaderGen(
				t,
				test.sharedMainName,
				test.ctrlNames,
				test.configReaderErrorVal,
			)
			pc, err := newConfig(fakeConfigReader)
			if test.newConfigError {
				AssertError(t, err, test.newConfigErrorVal)
			} else {
				AssertNoError(t, err)
			}
			assertPlatformConfig(t, pc, test.expected)
		})
	}
}
