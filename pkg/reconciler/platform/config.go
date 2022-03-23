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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func init() {
	RegisterFlags()
}

const (
	FlagControllers       string = "controllers"
	FlagSharedMainName    string = "unique-process-name"
	DefaultSharedMainName string = "tekton-operator"
)

var (
	ErrSharedMainNameEmpty = fmt.Errorf("sharedMainName cannot be empty string")
	ErrControllerNamesNil  = fmt.Errorf("ControllerNames slice should be non-nil")
	ctrlArgs               string
	processName            string
)

// RegisterFlags adds platform specific command line flags
// this logic is written in a separate function to make it convenient
// to write unit-tests
func RegisterFlags() {
	flag.StringVar(
		&ctrlArgs,
		FlagControllers,
		"",
		"comma separated list of names of controllers to be enabled (\"\" enables all controllers)",
	)

	// The role of this flag to make sure that instances of this process running as different
	// containers have unique "sharedMain Name"
	// The name has to be unique otherwise knative/pkg will consider the 2 containers as copies (like replicas in a deployment) of
	// same process as leader election and logging are setup using this "sharedMain Name"
	flag.StringVar(
		&processName,
		FlagSharedMainName,
		DefaultSharedMainName,
		"name of the sharedMain process used in leader election (unique among containers of same pod)")
}

// NewConfigFromFlags returns PlatformConfig created using
// inputs from command line flags
func NewConfigFromFlags() PlatformConfig {

	config, err := newConfig(flagsConfigReader)
	if err != nil {
		log.Fatalf("unable to read platform from flags: %v", err)
	}
	return config
}

// NewConfigFromEnv returns PlatformConfig created using
// inputs from environment variables
func NewConfigFromEnv() PlatformConfig {
	config, err := newConfig(envConfigReader)
	if err != nil {
		log.Fatalf("unable to read platform from env: %v", err)
	}
	return config
}

// envConfigReader of type 'configReader' is modular implementation of the logic
// to read platform specific inputs from environment variables
func envConfigReader(pc *PlatformConfig) error {
	ctrlArgs := os.Getenv(EnvControllerNames)
	c := os.Getenv(EnvSharedMainName)
	pc.SharedMainName = c
	pc.ControllerNames = stringToControllerNamesSlice(ctrlArgs)
	return nil
}

// flagsConfigReader 'configReader' is modular implementation of the logic
// to read platform specific inputs from command line flags
func flagsConfigReader(pc *PlatformConfig) error {
	flag.Parse()
	pc.SharedMainName = processName
	pc.ControllerNames = stringToControllerNamesSlice(ctrlArgs)
	return nil
}

// newConfig returns PlatformConfig created using inputs read by
// provided implementation of 'configReader'
func newConfig(inFn configReader) (PlatformConfig, error) {
	config := PlatformConfig{}
	err := inFn(&config)
	if err != nil {
		return PlatformConfig{}, err
	}
	if err := validateConfig(&config); err != nil {
		return PlatformConfig{}, err
	}
	return config, nil
}

// validateConfig does basic validation on platform specific configuration
func validateConfig(pc *PlatformConfig) error {
	violations := []string{}

	if len(pc.SharedMainName) == 0 {
		violations = append(violations, ErrSharedMainNameEmpty.Error())
	}
	// TODO: set a maximum length for pc.SharedMainName

	if pc.ControllerNames == nil {
		violations = append(violations, ErrControllerNamesNil.Error())
	}
	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf(strings.Join(violations, ","))
}

// stringToControllerNamesSlice returns a []ControllerName
// created from controllerNames in a comma separated string "ctrl1,ctrl2"
func stringToControllerNamesSlice(s string) []ControllerName {
	result := []ControllerName{}
	if len(s) == 0 {
		return result
	}
	for _, val := range strings.Split(s, ",") {
		result = append(result, ControllerName(val))
	}
	return result
}
