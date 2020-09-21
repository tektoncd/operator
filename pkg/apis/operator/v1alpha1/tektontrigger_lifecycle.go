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
	_ TektonComponentStatus = (*TektonTriggerStatus)(nil)

	triggersCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		DeploymentsAvailable,
		InstallSucceeded,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonTrigger
func (tp *TektonTrigger) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonTrigger)
}

// GetCondition returns the current condition of a given condition type
func (tps *TektonTriggerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return triggersCondSet.Manage(tps).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonTriggerStatus
func (tps *TektonTriggerStatus) InitializeConditions() {
	triggersCondSet.Manage(tps).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (tps *TektonTriggerStatus) IsReady() bool {
	return triggersCondSet.Manage(tps).IsHappy()
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (tps *TektonTriggerStatus) MarkInstallSucceeded() {
	triggersCondSet.Manage(tps).MarkTrue(InstallSucceeded)
	if tps.GetCondition(DependenciesInstalled).IsUnknown() {
		// Assume deps are installed if we're not sure
		tps.MarkDependenciesInstalled()
	}
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (tps *TektonTriggerStatus) MarkInstallFailed(msg string) {
	triggersCondSet.Manage(tps).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (tps *TektonTriggerStatus) MarkDeploymentsAvailable() {
	triggersCondSet.Manage(tps).MarkTrue(DeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (tps *TektonTriggerStatus) MarkDeploymentsNotReady() {
	triggersCondSet.Manage(tps).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (tps *TektonTriggerStatus) MarkDependenciesInstalled() {
	triggersCondSet.Manage(tps).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (tps *TektonTriggerStatus) MarkDependencyInstalling(msg string) {
	triggersCondSet.Manage(tps).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (tps *TektonTriggerStatus) MarkDependencyMissing(msg string) {
	triggersCondSet.Manage(tps).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

// GetVersion gets the currently installed version of the component.
func (tps *TektonTriggerStatus) GetVersion() string {
	return tps.Version
}

// SetVersion sets the currently installed version of the component.
func (tps *TektonTriggerStatus) SetVersion(version string) {
	tps.Version = version
}

// GetManifests gets the url links of the manifests.
func (tps *TektonTriggerStatus) GetManifests() []string {
	return tps.Manifests
}

// SetVersion sets the url links of the manifests.
func (tps *TektonTriggerStatus) SetManifests(manifests []string) {
	tps.Manifests = manifests
}
