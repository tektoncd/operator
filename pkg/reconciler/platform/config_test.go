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
	"flag"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/operator/pkg/reconciler/platform"
)

func TestNewConfigFromFlags(t *testing.T) {
	testCases := []struct {
		description            string
		processName            string
		ctrlNames              string
		setFlags               bool
		expectedPlatformConfig platform.PlatformConfig
	}{
		{
			description: "non-empty Input return non-empty PlatformConfig",
			processName: "abcd",
			ctrlNames:   "ctrl-1,ctrl-2",

			setFlags: true,
			expectedPlatformConfig: platform.PlatformConfig{
				Name: "",
				ControllerNames: []platform.ControllerName{
					platform.ControllerName("ctrl-1"),
					platform.ControllerName("ctrl-2"),
				},
				SharedMainName: "abcd",
			},
		},
		{
			description: "returns default Platform config when no flags are set",
			processName: "",
			ctrlNames:   "",

			setFlags: false,
			expectedPlatformConfig: platform.PlatformConfig{
				Name:            "",
				ControllerNames: []platform.ControllerName{},
				SharedMainName:  platform.DefaultSharedMainName,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ResetForTesting()
			if tc.setFlags {
				_ = flag.Set(platform.FlagControllers, tc.ctrlNames)
				_ = flag.Set(platform.FlagSharedMainName, tc.processName)
			}
			config := platform.NewConfigFromFlags()
			if diff := cmp.Diff(config, tc.expectedPlatformConfig); diff != "" {
				t.Errorf("expected platformConfig not equal to received platformConfig, diff: %s", diff)
			}

		})
	}
}

func ResetForTesting() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.Usage = func() {}
	flag.Usage = func() {}
	platform.RegisterFlags()
}
