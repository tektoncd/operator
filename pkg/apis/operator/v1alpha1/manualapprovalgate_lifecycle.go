/*
Copyright 2024 The Tekton Authors

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
	_ TektonComponentStatus = (*ManualApprovalGateStatus)(nil)

	magCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a ManualApprovalGate
func (mag *ManualApprovalGate) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindManualApprovalGate)
}

func (mag *ManualApprovalGate) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindManualApprovalGate)
}

// GetCondition returns the current condition of a given condition type
func (mag *ManualApprovalGateStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return magCondSet.Manage(mag).GetCondition(t)
}

// InitializeConditions initializes conditions of an ManualApprovalGateStatus
func (mag *ManualApprovalGateStatus) InitializeConditions() {
	magCondSet.Manage(mag).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (mag *ManualApprovalGateStatus) IsReady() bool {
	return magCondSet.Manage(mag).IsHappy()
}

func (mag *ManualApprovalGateStatus) MarkPreReconcilerComplete() {
	magCondSet.Manage(mag).MarkTrue(PreReconciler)
}

func (mag *ManualApprovalGateStatus) MarkInstallerSetAvailable() {
	magCondSet.Manage(mag).MarkTrue(InstallerSetAvailable)
}

func (mag *ManualApprovalGateStatus) MarkInstallerSetReady() {
	magCondSet.Manage(mag).MarkTrue(InstallerSetReady)
}

func (mag *ManualApprovalGateStatus) MarkPostReconcilerComplete() {
	magCondSet.Manage(mag).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (mag *ManualApprovalGateStatus) MarkDependenciesInstalled() {
	magCondSet.Manage(mag).MarkTrue(DependenciesInstalled)
}

func (mag *ManualApprovalGateStatus) MarkNotReady(msg string) {
	magCondSet.Manage(mag).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (mag *ManualApprovalGateStatus) MarkPreReconcilerFailed(msg string) {
	mag.MarkNotReady("PreReconciliation failed")
	magCondSet.Manage(mag).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (mag *ManualApprovalGateStatus) MarkInstallerSetNotAvailable(msg string) {
	mag.MarkNotReady("TektonInstallerSet not ready")
	magCondSet.Manage(mag).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (mag *ManualApprovalGateStatus) MarkInstallerSetNotReady(msg string) {
	mag.MarkNotReady("TektonInstallerSet not ready")
	magCondSet.Manage(mag).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (mag *ManualApprovalGateStatus) MarkPostReconcilerFailed(msg string) {
	mag.MarkNotReady("PostReconciliation failed")
	magCondSet.Manage(mag).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (mag *ManualApprovalGateStatus) MarkDependencyInstalling(msg string) {
	mag.MarkNotReady("Dependencies installing")
	magCondSet.Manage(mag).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (mag *ManualApprovalGateStatus) MarkDependencyMissing(msg string) {
	mag.MarkNotReady("Missing Dependencies for ManualApprovalGate")
	magCondSet.Manage(mag).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (mag *ManualApprovalGateStatus) GetTektonInstallerSet() string {
	return mag.TektonInstallerSet
}

func (mag *ManualApprovalGateStatus) SetTektonInstallerSet(installerSet string) {
	mag.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (mag *ManualApprovalGateStatus) GetVersion() string {
	return mag.Version
}

// SetVersion sets the currently installed version of the component.
func (mag *ManualApprovalGateStatus) SetVersion(version string) {
	mag.Version = version
}
