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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

var (
	_ TektonComponentStatus = (*TektonConfigStatus)(nil)

	configCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		DeploymentsAvailable,
		InstallSucceeded,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonConfig
func (tp *TektonConfig) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonConfig)
}

// GetCondition returns the current condition of a given condition type
func (tps *TektonConfigStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return configCondSet.Manage(tps).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonConfigStatus
func (tps *TektonConfigStatus) InitializeConditions() {
	configCondSet.Manage(tps).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (tps *TektonConfigStatus) IsReady() bool {
	return configCondSet.Manage(tps).IsHappy()
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (tps *TektonConfigStatus) MarkInstallSucceeded() {
	configCondSet.Manage(tps).MarkTrue(InstallSucceeded)
	if tps.GetCondition(DependenciesInstalled).IsUnknown() {
		// Assume deps are installed if we're not sure
		tps.MarkDependenciesInstalled()
	}
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (tps *TektonConfigStatus) MarkInstallFailed(msg string) {
	configCondSet.Manage(tps).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (tps *TektonConfigStatus) MarkDeploymentsAvailable() {
	configCondSet.Manage(tps).MarkTrue(DeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (tps *TektonConfigStatus) MarkDeploymentsNotReady() {
	configCondSet.Manage(tps).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (tps *TektonConfigStatus) MarkDependenciesInstalled() {
	configCondSet.Manage(tps).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (tps *TektonConfigStatus) MarkDependencyInstalling(msg string) {
	configCondSet.Manage(tps).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (tps *TektonConfigStatus) MarkDependencyMissing(msg string) {
	configCondSet.Manage(tps).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

// GetVersion gets the currently installed version of the component.
func (tps *TektonConfigStatus) GetVersion() string {
	return tps.Version
}

// SetVersion sets the currently installed version of the component.
func (tps *TektonConfigStatus) SetVersion(version string) {
	tps.Version = version
}

// GetManifests gets the url links of the manifests.
func (tps *TektonConfigStatus) GetManifests() []string {
	return tps.Manifests
}

// SetVersion sets the url links of the manifests.
func (tps *TektonConfigStatus) SetManifests(manifests []string) {
	tps.Manifests = manifests
}
