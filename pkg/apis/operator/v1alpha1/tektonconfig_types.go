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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TektonConfig is the Schema for the TektonConfigs API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonConfigSpec   `json:"spec,omitempty"`
	Status TektonConfigStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonConfig) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

// GetStatus implements TektonComponent
func (tp *TektonConfig) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// Prune defines the pruner
type Prune struct {
	// The resources which need to be pruned
	Resources []string `json:"resources,omitempty"`
	// The number of resource to keep
	// You dont want to delete all the pipelinerun/taskrun's by a cron
	// +optional
	Keep *uint `json:"keep,omitempty"`
	// KeepSince keeps the resources younger than the specified value
	// Its value is taken in minutes
	// +optional
	KeepSince *uint `json:"keep-since,omitempty"`
	// How frequent pruning should happen
	Schedule string `json:"schedule,omitempty"`
}

func (p Prune) IsEmpty() bool {
	return reflect.DeepEqual(p, Prune{})
}

// TektonConfigSpec defines the desired state of TektonConfig
type TektonConfigSpec struct {
	Profile string `json:"profile,omitempty"`
	// Config holds the configuration for resources created by TektonConfig
	// +optional
	Config Config `json:"config,omitempty"`
	// Pruner holds the prune config
	// +optional
	Pruner     Prune `json:"pruner,omitempty"`
	CommonSpec `json:",inline"`
	// Addon holds the addons config
	// +optional
	Addon Addon `json:"addon,omitempty"`
	// Hub holds the hub config
	// +optional
	Hub Hub `json:"hub,omitempty"`
	// Pipeline holds the customizable option for pipeline component
	// +optional
	Pipeline Pipeline `json:"pipeline,omitempty"`
	// Trigger holds the customizable option for triggers component
	// +optional
	Trigger Trigger `json:"trigger,omitempty"`
	// Dashboard holds the customizable options for dashboards component
	// +optional
	Dashboard Dashboard `json:"dashboard,omitempty"`
	// Params is the list of params passed for all platforms
	// +optional
	Params []Param `json:"params,omitempty"`
	// Platforms allows configuring platform specific configurations
	// +optional
	Platforms Platforms `json:"platforms,omitempty"`
}

// TektonConfigStatus defines the observed state of TektonConfig
type TektonConfigStatus struct {
	duckv1.Status `json:",inline"`

	// The profile installed
	// +optional
	Profile string `json:"profile,omitempty"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name
	// +optional
	TektonInstallerSet map[string]string `json:"tektonInstallerSets,omitempty"`
}

func (in *TektonConfigStatus) MarkInstallerSetReady() {
	//TODO implement me
	panic("implement me")
}

func (in *TektonConfigStatus) MarkInstallerSetNotReady(s string) {
	//TODO implement me
	panic("implement me")
}

func (in *TektonConfigStatus) MarkInstallerSetAvailable() {
	//TODO implement me
	panic("implement me")
}

func (in *TektonConfigStatus) MarkPreReconcilerFailed(s string) {
	//TODO implement me
	panic("implement me")
}

func (in *TektonConfigStatus) MarkPostReconcilerFailed(s string) {
	//TODO implement me
	panic("implement me")
}

// TektonConfigList contains a list of TektonConfig
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonConfig `json:"items"`
}

type Config struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	// PriorityClassName holds the priority class to be set to pod template
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// HighAvailability allows specification of HA control plane.
	// +optional
	HighAvailability HighAvailability `json:"highAvailability,omitempty"`
	// DeploymentOverride overrides Deployment configurations such as resource.
	// +optional
	DeploymentOverride []DeploymentOverride `json:"deployments,omitempty"`
}

// HighAvailability specifies options for deploying Tekton Pipeline Deployment
type HighAvailability struct {
	// Replicas is the number of replicas that HA parts of the control plane
	// will be scaled to.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// DeploymentOverride specifies resource override for deployment
type DeploymentOverride struct {
	// Name is the name of the deployment to override.
	Name string `json:"name"`
	// Resources overrides resources for the containers.
	// +optional
	Containers []ContainerOverride `json:"containers,omitempty"`
	// Replicas is the number of replicas that this deployment will scaled to.
	// It has a higher priority than HighAvailability.Replicas
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// ContainerOverride enables the user to override any container's
// configuration specified in the embedded manifest
type ContainerOverride struct {
	// Name represent container name
	Name string `json:"name"`
	// Resource represent the desired ResourceRequirements
	// +optional
	Resource corev1.ResourceRequirements `json:"resource,omitempty"`
	// Env represent the env that will replace the existing one or append if not existed
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// Args represent the args will append to the existing args
	// +optional
	Args []string `json:"args,omitempty"`
}

type Platforms struct {
	// OpenShift allows configuring openshift specific components and configurations
	// +optional
	OpenShift OpenShift `json:"openshift,omitempty"`
}
