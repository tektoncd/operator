/*
Copyright 2026 The Tekton Authors

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
	_ TektonComponentStatus = (*ShipwrightBuildStatus)(nil)

	ShipwrightBuildCondSet = apis.NewLivingConditionSet(
		PreReconciler,
		DependenciesInstalled,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of an ShipwrightBuild
func (sb *ShipwrightBuild) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindShipwrightBuild)
}

func (sb *ShipwrightBuild) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindShipwrightBuild)
}

// GetCondition returns the current condition of a given condition type
func (status *ShipwrightBuildStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return ShipwrightBuildCondSet.Manage(status).GetCondition(t)
}

// InitializeConditions initializes conditions of an ShipwrightBuildStatus
func (status *ShipwrightBuildStatus) InitializeConditions() {
	ShipwrightBuildCondSet.Manage(status).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (status *ShipwrightBuildStatus) IsReady() bool {
	return ShipwrightBuildCondSet.Manage(status).IsHappy()
}

func (status *ShipwrightBuildStatus) MarkNotReady(msg string) {
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (status *ShipwrightBuildStatus) MarkPreReconcilerComplete() {
	ShipwrightBuildCondSet.Manage(status).MarkTrue(PreReconciler)
}

func (status *ShipwrightBuildStatus) MarkInstallerSetAvailable() {
	ShipwrightBuildCondSet.Manage(status).MarkTrue(InstallerSetAvailable)
}

func (status *ShipwrightBuildStatus) MarkInstallerSetReady() {
	ShipwrightBuildCondSet.Manage(status).MarkTrue(InstallerSetReady)
}

func (status *ShipwrightBuildStatus) MarkInstallerSetNotAvailable(msg string) {
	status.MarkNotReady("InstallerSet not ready")
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (status *ShipwrightBuildStatus) MarkInstallerSetNotReady(msg string) {
	status.MarkNotReady("InstallerSet not ready")
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (status *ShipwrightBuildStatus) MarkPostReconcilerComplete() {
	ShipwrightBuildCondSet.Manage(status).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (status *ShipwrightBuildStatus) MarkDependenciesInstalled() {
	ShipwrightBuildCondSet.Manage(status).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false
func (status *ShipwrightBuildStatus) MarkDependencyInstalling(msg string) {
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false
func (status *ShipwrightBuildStatus) MarkDependencyMissing(msg string) {
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (status *ShipwrightBuildStatus) MarkPreReconcilerFailed(msg string) {
	status.MarkNotReady("PreReconciliation failed")
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		PreReconciler,
		"Error",
		msg,
	)
}

func (status *ShipwrightBuildStatus) MarkPostReconcilerFailed(msg string) {
	status.MarkNotReady("PostReconciliation failed")
	ShipwrightBuildCondSet.Manage(status).MarkFalse(
		PostReconciler,
		"Error",
		msg,
	)
}


func (status *ShipwrightBuildStatus) GetTektonInstallerSet() string {
	return status.TektonInstallerSet
}

func (status *ShipwrightBuildStatus) SetTektonInstallerSet(tektonInstallerSet string) {
	status.TektonInstallerSet = tektonInstallerSet
}

// GetVersion gets the currently installed version of the component.
func (status *ShipwrightBuildStatus) GetVersion() string {
	return status.Version
}

// SetVersion sets the currently installed version of the component.
func (status *ShipwrightBuildStatus) SetVersion(version string) {
	status.Version = version
}
