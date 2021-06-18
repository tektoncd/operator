/*
Copyright 2020 The Tekton Authors

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

const (
	// DependenciesInstalled is a Condition indicating that potential dependencies have
	// been installed correctly.
	DependenciesInstalled apis.ConditionType = "DependenciesInstalled"
	// InstallSucceeded is a Condition indiciating that the installation of the component
	// itself has been successful.
	InstallSucceeded apis.ConditionType = "InstallSucceeded"
	// DeploymentsAvailable is a Condition indicating whether or not the Deployments of
	// the respective component have come up successfully.
	DeploymentsAvailable apis.ConditionType = "DeploymentsAvailable"
)

// TektonComponent is a common interface for accessing meta, spec and status of all known types.
type TektonComponent interface {
	metav1.Object
	schema.ObjectKind

	// GetSpec returns the common spec for all known types.
	GetSpec() TektonComponentSpec
	// GetStatus returns the common status of all known types.
	GetStatus() TektonComponentStatus
}

// TektonComponentSpec is a common interface for accessing the common spec of all known types.
type TektonComponentSpec interface {
	// GetTargetNamespace gets the version to be installed
	GetTargetNamespace() string
}

// TektonComponentStatus is a common interface for status mutations of all known types.
type TektonComponentStatus interface {
	// MarkInstallSucceeded marks the InstallationSucceeded status as true.
	MarkInstallSucceeded()
	// MarkInstallFailed marks the InstallationSucceeded status as false with the given
	// message.
	MarkInstallFailed(msg string)

	// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
	MarkDeploymentsAvailable()
	// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
	// it's waiting for deployments.
	MarkDeploymentsNotReady()

	// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
	MarkDependenciesInstalled()
	// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
	// given message.
	MarkDependencyInstalling(msg string)
	// MarkDependencyMissing marks the DependenciesInstalled status as false with the
	// given message.
	MarkDependencyMissing(msg string)

	// GetVersion gets the currently installed version of the component.
	GetVersion() string
	// SetVersion sets the currently installed version of the component.
	SetVersion(version string)

	// GetManifests gets the url links of the manifests
	GetManifests() []string

	// IsReady return true if all conditions are satisfied
	IsReady() bool
}

// CommonSpec unifies common fields and functions on the Spec.
type CommonSpec struct {
	// TargetNamespace is where resources will be installed
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`
}

// GetTargetNamespace implements KComponentSpec.
func (c *CommonSpec) GetTargetNamespace() string {
	return c.TargetNamespace
}

// Param declares an string value to use for the parameter called name.
type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ParamValue defines a default value and possible values for a param
type ParamValue struct {
	Default  string
	Possible []string
}

// ParseParams returns the params array as map
func ParseParams(params []Param) map[string]string {
	paramsMap := map[string]string{}
	for _, p := range params {
		paramsMap[p.Name] = p.Value
	}
	return paramsMap
}
