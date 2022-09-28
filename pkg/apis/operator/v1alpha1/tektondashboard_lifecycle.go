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
	_ TektonComponentStatus = (*TektonDashboardStatus)(nil)

	dashboardCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonDashboard
func (td *TektonDashboard) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonDashboard)
}

func (td *TektonDashboard) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonDashboard)
}

// GetCondition returns the current condition of a given condition type
func (tds *TektonDashboardStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return dashboardCondSet.Manage(tds).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonDashboardStatus
func (tds *TektonDashboardStatus) InitializeConditions() {
	dashboardCondSet.Manage(tds).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (tds *TektonDashboardStatus) IsReady() bool {
	return dashboardCondSet.Manage(tds).IsHappy()
}

func (tds *TektonDashboardStatus) MarkPreReconcilerComplete() {
	dashboardCondSet.Manage(tds).MarkTrue(PreReconciler)
}

func (tds *TektonDashboardStatus) MarkInstallerSetAvailable() {
	dashboardCondSet.Manage(tds).MarkTrue(InstallerSetAvailable)
}

func (tds *TektonDashboardStatus) MarkInstallerSetReady() {
	dashboardCondSet.Manage(tds).MarkTrue(InstallerSetReady)
}

func (tds *TektonDashboardStatus) MarkPostReconcilerComplete() {
	dashboardCondSet.Manage(tds).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (tds *TektonDashboardStatus) MarkDependenciesInstalled() {
	dashboardCondSet.Manage(tds).MarkTrue(DependenciesInstalled)
}

func (tds *TektonDashboardStatus) MarkNotReady(msg string) {
	dashboardCondSet.Manage(tds).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tds *TektonDashboardStatus) MarkPreReconcilerFailed(msg string) {
	tds.MarkNotReady("PreReconciliation failed")
	dashboardCondSet.Manage(tds).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tds *TektonDashboardStatus) MarkInstallerSetNotAvailable(msg string) {
	tds.MarkNotReady("TektonInstallerSet not ready")
	dashboardCondSet.Manage(tds).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tds *TektonDashboardStatus) MarkInstallerSetNotReady(msg string) {
	tds.MarkNotReady("TektonInstallerSet not ready")
	dashboardCondSet.Manage(tds).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tds *TektonDashboardStatus) MarkPostReconcilerFailed(msg string) {
	tds.MarkNotReady("PostReconciliation failed")
	dashboardCondSet.Manage(tds).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (tds *TektonDashboardStatus) MarkDependencyInstalling(msg string) {
	tds.MarkNotReady("Dependencies installing")
	dashboardCondSet.Manage(tds).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (tds *TektonDashboardStatus) MarkDependencyMissing(msg string) {
	tds.MarkNotReady("Missing Dependencies for TektonDashboard")
	dashboardCondSet.Manage(tds).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (tds *TektonDashboardStatus) GetTektonInstallerSet() string {
	return tds.TektonInstallerSet
}

func (tds *TektonDashboardStatus) SetTektonInstallerSet(installerSet string) {
	tds.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (tds *TektonDashboardStatus) GetVersion() string {
	return tds.Version
}

// SetVersion sets the currently installed version of the component.
func (tds *TektonDashboardStatus) SetVersion(version string) {
	tds.Version = version
}
