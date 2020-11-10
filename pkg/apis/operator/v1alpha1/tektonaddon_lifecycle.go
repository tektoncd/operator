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
	_             TektonComponentStatus = (*TektonAddonStatus)(nil)
	addonsCondSet                       = apis.NewLivingConditionSet(
		DependenciesInstalled,
		DeploymentsAvailable,
		InstallSucceeded,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonAddon
func (tp *TektonAddon) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonAddon)
}

// GetCondition returns the current condition of a given condition type
func (tps *TektonAddonStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return addonsCondSet.Manage(tps).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonAddonStatus
func (tps *TektonAddonStatus) InitializeConditions() {
	addonsCondSet.Manage(tps).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (tps *TektonAddonStatus) IsReady() bool {
	return addonsCondSet.Manage(tps).IsHappy()
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (tps *TektonAddonStatus) MarkInstallSucceeded() {
	addonsCondSet.Manage(tps).MarkTrue(InstallSucceeded)
	if tps.GetCondition(DependenciesInstalled).IsUnknown() {
		// Assume deps are installed if we're not sure
		tps.MarkDependenciesInstalled()
	}
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (tps *TektonAddonStatus) MarkInstallFailed(msg string) {
	addonsCondSet.Manage(tps).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (tps *TektonAddonStatus) MarkDeploymentsAvailable() {
	addonsCondSet.Manage(tps).MarkTrue(DeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (tps *TektonAddonStatus) MarkDeploymentsNotReady() {
	addonsCondSet.Manage(tps).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (tps *TektonAddonStatus) MarkDependenciesInstalled() {
	addonsCondSet.Manage(tps).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (tps *TektonAddonStatus) MarkDependencyInstalling(msg string) {
	addonsCondSet.Manage(tps).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (tps *TektonAddonStatus) MarkDependencyMissing(msg string) {
	addonsCondSet.Manage(tps).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

// GetVersion gets the currently installed version of the component.
func (tps *TektonAddonStatus) GetVersion() string {
	return tps.Version
}

// SetVersion sets the currently installed version of the component.
func (tps *TektonAddonStatus) SetVersion(version string) {
	tps.Version = version
}

// GetManifests gets the url links of the manifests.
func (tps *TektonAddonStatus) GetManifests() []string {
	return tps.Manifests
}

// SetVersion sets the url links of the manifests.
func (tps *TektonAddonStatus) SetManifests(manifests []string) {
	tps.Manifests = manifests
}
