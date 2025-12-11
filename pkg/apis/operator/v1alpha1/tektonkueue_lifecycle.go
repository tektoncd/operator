/*
Copyright 2025 The Tekton Authors

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
	_ TektonComponentStatus = (*TektonKueueStatus)(nil)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonKueue
func (kueue *TektonKueue) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonKueue)
}

func (kueue *TektonKueue) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonKueue)
}

// GetCondition returns the current condition of a given condition type
func (kueue *TektonKueueStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return condSet.Manage(kueue).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonKueueStatus
func (kueue *TektonKueueStatus) InitializeConditions() {
	condSet.Manage(kueue).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (kueue *TektonKueueStatus) IsReady() bool {
	return condSet.Manage(kueue).IsHappy()
}

func (kueue *TektonKueueStatus) MarkPreReconcilerComplete() {
	condSet.Manage(kueue).MarkTrue(PreReconciler)
}

func (kueue *TektonKueueStatus) MarkInstallerSetAvailable() {
	condSet.Manage(kueue).MarkTrue(InstallerSetAvailable)
}

func (kueue *TektonKueueStatus) MarkInstallerSetReady() {
	condSet.Manage(kueue).MarkTrue(InstallerSetReady)
}

func (kueue *TektonKueueStatus) MarkPostReconcilerComplete() {
	condSet.Manage(kueue).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (kueue *TektonKueueStatus) MarkDependenciesInstalled() {
	condSet.Manage(kueue).MarkTrue(DependenciesInstalled)
}

func (kueue *TektonKueueStatus) MarkNotReady(msg string) {
	condSet.Manage(kueue).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (kueue *TektonKueueStatus) MarkPreReconcilerFailed(msg string) {
	kueue.MarkNotReady("PreReconciliation failed")
	condSet.Manage(kueue).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (kueue *TektonKueueStatus) MarkInstallerSetNotAvailable(msg string) {
	kueue.MarkNotReady("TektonInstallerSet not ready")
	condSet.Manage(kueue).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (kueue *TektonKueueStatus) MarkInstallerSetNotReady(msg string) {
	kueue.MarkNotReady("TektonInstallerSet not ready")
	condSet.Manage(kueue).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (kueue *TektonKueueStatus) MarkPostReconcilerFailed(msg string) {
	kueue.MarkNotReady("PostReconciliation failed")
	condSet.Manage(kueue).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (kueue *TektonKueueStatus) MarkDependencyInstalling(msg string) {
	kueue.MarkNotReady("Dependencies installing")
	condSet.Manage(kueue).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (kueue *TektonKueueStatus) MarkDependencyMissing(msg string) {
	kueue.MarkNotReady("Missing Dependencies for TektonKueue")
	condSet.Manage(kueue).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (kueue *TektonKueueStatus) GetTektonInstallerSet() string {
	return kueue.TektonInstallerSet
}

func (kueue *TektonKueueStatus) SetTektonInstallerSet(installerSet string) {
	kueue.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (kueue *TektonKueueStatus) GetVersion() string {
	return kueue.Version
}

// SetVersion sets the currently installed version of the component.
func (kueue *TektonKueueStatus) SetVersion(version string) {
	kueue.Version = version
}
