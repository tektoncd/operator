/*
Copyright 2023 The Tekton Authors

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

package utils

import (
	"os"
	"strings"
)

const (
	// log level of the logger used in e2e tests
	ENV_LOG_LEVEL = "LOG_LEVEL"
	ENV_TARGET    = "TARGET"
	ENV_PLATFORM  = "PLATFORM"

	// os/architecture types
	LinuxPPC64LE = "linux/ppc64le"
	LinuxS390X   = "linux/s390x"
)

func GetEnvironment(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// in the development package "PLATFORM" is used to know about cluster environment, ie: OpenShift or kubernetes (v1alpha1.IsOpenShiftPlatform())
// but in test uses "TARGET" is used for the same and "PLATFORM" refers os/arch in tests
func IsOpenShift() bool {
	return strings.ToLower(os.Getenv(ENV_TARGET)) == "openshift"
}

func GetOSAndArchitecture() string {
	return os.Getenv(ENV_PLATFORM)
}
