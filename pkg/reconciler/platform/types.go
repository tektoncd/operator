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
	"knative.dev/pkg/injection"
)

type configReader func(config *PlatformConfig) error

// PlatformConfig defines basic configuration that
// all platforms should support
type PlatformConfig struct {
	Name            string
	ControllerNames []ControllerName
	SharedMainName  string
}

// PlatformNameKey is defines a 'key' for adding platform name to an instance of context.Context
type PlatformNameKey struct{}

// ControllerName defines a name given to a controller(reconciler) in a platform
type ControllerName string

// ControllerMap defines map that maps a name given to a controller(reconciler) to its injection.ControllerConstructor
type ControllerMap map[ControllerName]injection.NamedControllerConstructor

// ControllerNames returns a []string of names of all controllers (reconciers)
// supported in a given ControlelrMap
func (cm ControllerMap) ControllerNames() []string {
	result := []string{}
	for _, namedCtrl := range cm {
		result = append(result, namedCtrl.Name)
	}
	return result
}

// ControllerConstructor returns a []injection.ControllerConstructor of all controllers (reconciers)
// supported in a given ControlelrMap
// Some versions of sharedMain functions in knative/pkg expect
// a variadic list of supported cotrollers as []injection.ControllerConstructor
func (cm ControllerMap) ControllerConstructors() []injection.ControllerConstructor {
	result := []injection.ControllerConstructor{}
	for _, namedCtrl := range cm {
		result = append(result, namedCtrl.ControllerConstructor)
	}
	return result
}

// ControllerConstructor returns a []injection.NamedControllerConstructor of all controllers (reconciers)
// supported in a given ControlelrMap
// Some versions of sharedMain functions in knative/pkg expect
// a variadic list of supported cotrollers as []injection.NamedControllerConstructor
func (cm ControllerMap) NamedControllerConstructors() []injection.NamedControllerConstructor {
	result := []injection.NamedControllerConstructor{}
	for _, namedCtrl := range cm {
		result = append(result, namedCtrl)
	}
	return result
}

// Platform defines a Kubernetes platform (Vanila Kubernetes, OpenShift...)
type Platform interface {
	PlatformParams() PlatformConfig
	AllSupportedControllers() ControllerMap
}
