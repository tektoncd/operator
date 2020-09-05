/*
Copyright 2019 The Knative Authors

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
	_ KComponentStatus = (*KnativeServingStatus)(nil)

	servingCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		DeploymentsAvailable,
		InstallSucceeded,
		VersionMigrationEligible,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a KnativeServing
func (ks *KnativeServing) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindKnativeServing)
}

// GetCondition returns the current condition of a given condition type
func (is *KnativeServingStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return servingCondSet.Manage(is).GetCondition(t)
}

// InitializeConditions initializes conditions of an KnativeServingStatus
func (is *KnativeServingStatus) InitializeConditions() {
	servingCondSet.Manage(is).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (is *KnativeServingStatus) IsReady() bool {
	return servingCondSet.Manage(is).IsHappy()
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (is *KnativeServingStatus) MarkInstallSucceeded() {
	servingCondSet.Manage(is).MarkTrue(InstallSucceeded)
	if is.GetCondition(DependenciesInstalled).IsUnknown() {
		// Assume deps are installed if we're not sure
		is.MarkDependenciesInstalled()
	}
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (is *KnativeServingStatus) MarkInstallFailed(msg string) {
	servingCondSet.Manage(is).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

// MarkVersionMigrationEligible marks the VersionMigrationEligible status as false with given message.
func (is *KnativeServingStatus) MarkVersionMigrationEligible() {
	servingCondSet.Manage(is).MarkTrue(VersionMigrationEligible)
}

// MarkVersionMigrationNotEligible marks the DeploymentsAvailable status as true.
func (is *KnativeServingStatus) MarkVersionMigrationNotEligible(msg string) {
	servingCondSet.Manage(is).MarkFalse(
		VersionMigrationEligible,
		"Error",
		"Version migration is not eligible with message: %s", msg)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (is *KnativeServingStatus) MarkDeploymentsAvailable() {
	servingCondSet.Manage(is).MarkTrue(DeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (is *KnativeServingStatus) MarkDeploymentsNotReady() {
	servingCondSet.Manage(is).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (is *KnativeServingStatus) MarkDependenciesInstalled() {
	servingCondSet.Manage(is).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (is *KnativeServingStatus) MarkDependencyInstalling(msg string) {
	servingCondSet.Manage(is).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (is *KnativeServingStatus) MarkDependencyMissing(msg string) {
	servingCondSet.Manage(is).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

// GetVersion gets the currently installed version of the component.
func (is *KnativeServingStatus) GetVersion() string {
	return is.Version
}

// SetVersion sets the currently installed version of the component.
func (is *KnativeServingStatus) SetVersion(version string) {
	is.Version = version
}

// GetManifests gets the url links of the manifests.
func (is *KnativeServingStatus) GetManifests() []string {
	return is.Manifests
}

// SetVersion sets the url links of the manifests.
func (is *KnativeServingStatus) SetManifests(manifests []string) {
	is.Manifests = manifests
}
