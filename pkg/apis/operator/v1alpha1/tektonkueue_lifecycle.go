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
	_ TektonComponentStatus = (*TektonKueueStatus)(nil)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonKueue
func (Kueue *TektonKueue) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonKueue)
}

func (Kueue *TektonKueue) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonKueue)
}

// GetCondition returns the current condition of a given condition type
func (Kueue *TektonKueueStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return condSet.Manage(Kueue).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonKueueStatus
func (Kueue *TektonKueueStatus) InitializeConditions() {
	condSet.Manage(Kueue).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (Kueue *TektonKueueStatus) IsReady() bool {
	return condSet.Manage(Kueue).IsHappy()
}

func (Kueue *TektonKueueStatus) MarkPreReconcilerComplete() {
	condSet.Manage(Kueue).MarkTrue(PreReconciler)
}

func (Kueue *TektonKueueStatus) MarkInstallerSetAvailable() {
	condSet.Manage(Kueue).MarkTrue(InstallerSetAvailable)
}

func (Kueue *TektonKueueStatus) MarkInstallerSetReady() {
	condSet.Manage(Kueue).MarkTrue(InstallerSetReady)
}

func (Kueue *TektonKueueStatus) MarkPostReconcilerComplete() {
	condSet.Manage(Kueue).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (Kueue *TektonKueueStatus) MarkDependenciesInstalled() {
	condSet.Manage(Kueue).MarkTrue(DependenciesInstalled)
}

func (Kueue *TektonKueueStatus) MarkNotReady(msg string) {
	condSet.Manage(Kueue).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (Kueue *TektonKueueStatus) MarkPreReconcilerFailed(msg string) {
	Kueue.MarkNotReady("PreReconciliation failed")
	condSet.Manage(Kueue).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (Kueue *TektonKueueStatus) MarkInstallerSetNotAvailable(msg string) {
	Kueue.MarkNotReady("TektonKueue not ready")
	condSet.Manage(Kueue).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (Kueue *TektonKueueStatus) MarkInstallerSetNotReady(msg string) {
	Kueue.MarkNotReady("TektonKueue not ready")
	condSet.Manage(Kueue).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (Kueue *TektonKueueStatus) MarkPostReconcilerFailed(msg string) {
	Kueue.MarkNotReady("PostReconciliation failed")
	condSet.Manage(Kueue).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (Kueue *TektonKueueStatus) MarkDependencyInstalling(msg string) {
	Kueue.MarkNotReady("Dependencies installing")
	condSet.Manage(Kueue).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (Kueue *TektonKueueStatus) MarkDependencyMissing(msg string) {
	Kueue.MarkNotReady("Missing Dependencies for TektonKueue")
	condSet.Manage(Kueue).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (Kueue *TektonKueueStatus) GetTektonKueue() string {
	return Kueue.TektonKueue
}

func (Kueue *TektonKueueStatus) SetTektonKueue(installerSet string) {
	Kueue.TektonKueue = installerSet
}

// GetVersion gets the currently installed version of the component.
func (Kueue *TektonKueueStatus) GetVersion() string {
	return Kueue.Version
}

// SetVersion sets the currently installed version of the component.
func (Kueue *TektonKueueStatus) SetVersion(version string) {
	Kueue.Version = version
}
