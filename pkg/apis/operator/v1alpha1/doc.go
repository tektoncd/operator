/*
Copyright 2019 The Tekton Authors
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
	corev1 "k8s.io/api/core/v1"
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
	GetSetatus() TektonComponentStatus
}

// TektonComponentSpec is a common interface for accessing the common spec of all known types.
type TektonComponentSpec interface {
	// GetConfig returns means to override entries in upstream configmaps.
	GetConfig() ConfigMapData
	// GetRegistry returns means to override deployment images.
	GetRegistry() *Registry
	// GetResources returns a list of container resource overrides.
	GetResources() []ResourceRequirementsOverride
	// GetVersion gets the version to be installed
	GetVersion() string
	// GetManifests gets the list of manifests, which should ultimately be installed
	GetManifests() []Manifest
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

	// MarkVersionMigrationEligible marks the VersionMigrationEligible status as true.
	MarkVersionMigrationEligible()
	// MarkVersionMigrationNotEligible marks the VersionMigrationEligible status as false with
	// the given message.
	MarkVersionMigrationNotEligible(msg string)

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
	// SetManifests sets the url links of the manifests
	SetManifests(manifests []string)

	// IsReady return true if all conditions are satisfied
	IsReady() bool
}

// CommonSpec unifies common fields and functions on the Spec.
type CommonSpec struct {
	// A means to override the corresponding entries in the upstream configmaps
	// +optional
	Config ConfigMapData `json:"config,omitempty"`

	// A means to override the corresponding deployment images in the upstream.
	// If no registry is provided, the knative release images will be used.
	// +optional
	Registry Registry `json:"registry,omitempty"`

	// Override containers' resource requirements
	// +optional
	Resources []ResourceRequirementsOverride `json:"resources,omitempty"`

	// Override containers' resource requirements
	// +optional
	Version string `json:"version,omitempty"`

	// A means to specify the manifests to install
	// +optional
	Manifests []Manifest `json:"manifests,omitempty"`
}

// GetConfig implements KComponentSpec.
func (c *CommonSpec) GetConfig() ConfigMapData {
	return c.Config
}

// GetRegistry implements KComponentSpec.
func (c *CommonSpec) GetRegistry() *Registry {
	return &c.Registry
}

// GetResources implements KComponentSpec.
func (c *CommonSpec) GetResources() []ResourceRequirementsOverride {
	return c.Resources
}

// GetVersion implements KComponentSpec.
func (c *CommonSpec) GetVersion() string {
	return c.Version
}

// GetManifests implements KComponentSpec.
func (c *CommonSpec) GetManifests() []Manifest {
	return c.Manifests
}

// ConfigMapData is a nested map of maps representing all upstream ConfigMaps. The first
// level key is the key to the ConfigMap itself (i.e. "logging") while the second level
// is the data to be filled into the respective ConfigMap.
type ConfigMapData map[string]map[string]string

// Registry defines image overrides of knative images.
// This affects both apps/v1.Deployment and caching.internal.knative.dev/v1alpha1.Image.
// The default value is used as a default format to override for all knative deployments.
// The override values are specific to each knative deployment.
type Registry struct {
	// The default image reference template to use for all knative images.
	// It takes the form of example-registry.io/custom/path/${NAME}:custom-tag
	// ${NAME} will be replaced by the deployment container name, or caching.internal.knative.dev/v1alpha1/Image name.
	// +optional
	Default string `json:"default,omitempty"`

	// A map of a container name or image name to the full image location of the individual knative image.
	// +optional
	Override map[string]string `json:"override,omitempty"`

	// A list of secrets to be used when pulling the knative images. The secret must be created in the
	// same namespace as the knative-serving deployments, and not the namespace of this resource.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// ResourceRequirementsOverride enables the user to override any container's
// resource requests/limits specified in the embedded manifest
type ResourceRequirementsOverride struct {
	// The container name
	Container string `json:"container"`
	// The desired ResourceRequirements
	corev1.ResourceRequirements
}

// Manifest enables the user to specify the links to the manifests' URLs
type Manifest struct {
	// The link of the manifest URL
	Url string `json:"URL"`
}
