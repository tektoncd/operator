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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

var (
	_ TektonComponentStatus = (*TektonPipelineStatus)(nil)

	pipelineCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		DeploymentsAvailable,
		InstallSucceeded,
		VersionMigrationEligible,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonPipeline
func (ks *TektonPipeline) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonPipeline)
}

// GetCondition returns the current condition of a given condition type
func (is *TektonPipelineStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pipelineCondSet.Manage(is).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonPipelineStatus
func (is *TektonPipelineStatus) InitializeConditions() {
	pipelineCondSet.Manage(is).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (is *TektonPipelineStatus) IsReady() bool {
	return pipelineCondSet.Manage(is).IsHappy()
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (is *TektonPipelineStatus) MarkInstallSucceeded() {
	pipelineCondSet.Manage(is).MarkTrue(InstallSucceeded)
	if is.GetCondition(DependenciesInstalled).IsUnknown() {
		// Assume deps are installed if we're not sure
		is.MarkDependenciesInstalled()
	}
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (is *TektonPipelineStatus) MarkInstallFailed(msg string) {
	pipelineCondSet.Manage(is).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

// MarkVersionMigrationEligible marks the VersionMigrationEligible status as false with given message.
func (is *TektonPipelineStatus) MarkVersionMigrationEligible() {
	pipelineCondSet.Manage(is).MarkTrue(VersionMigrationEligible)
}

// MarkVersionMigrationNotEligible marks the DeploymentsAvailable status as true.
func (is *TektonPipelineStatus) MarkVersionMigrationNotEligible(msg string) {
	pipelineCondSet.Manage(is).MarkFalse(
		VersionMigrationEligible,
		"Error",
		"Version migration is not eligible with message: %s", msg)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (is *TektonPipelineStatus) MarkDeploymentsAvailable() {
	pipelineCondSet.Manage(is).MarkTrue(DeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (is *TektonPipelineStatus) MarkDeploymentsNotReady() {
	pipelineCondSet.Manage(is).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (is *TektonPipelineStatus) MarkDependenciesInstalled() {
	pipelineCondSet.Manage(is).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (is *TektonPipelineStatus) MarkDependencyInstalling(msg string) {
	pipelineCondSet.Manage(is).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (is *TektonPipelineStatus) MarkDependencyMissing(msg string) {
	pipelineCondSet.Manage(is).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

// GetVersion gets the currently installed version of the component.
func (is *TektonPipelineStatus) GetVersion() string {
	return is.Version
}

// SetVersion sets the currently installed version of the component.
func (is *TektonPipelineStatus) SetVersion(version string) {
	is.Version = version
}

// GetManifests gets the url links of the manifests.
func (is *TektonPipelineStatus) GetManifests() []string {
	return is.Manifests
}

// SetVersion sets the url links of the manifests.
func (is *TektonPipelineStatus) SetManifests(manifests []string) {
	is.Manifests = manifests
}
