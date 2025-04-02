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
	_ TektonComponentStatus = (*TektonPrunerStatus)(nil)

	condSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonPruner
func (pruner *TektonPruner) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonPruner)
}

func (pruner *TektonPruner) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonPruner)
}

// GetCondition returns the current condition of a given condition type
func (pruner *TektonPrunerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return condSet.Manage(pruner).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonPrunerStatus
func (pruner *TektonPrunerStatus) InitializeConditions() {
	condSet.Manage(pruner).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (pruner *TektonPrunerStatus) IsReady() bool {
	return condSet.Manage(pruner).IsHappy()
}

func (pruner *TektonPrunerStatus) MarkPreReconcilerComplete() {
	condSet.Manage(pruner).MarkTrue(PreReconciler)
}

func (pruner *TektonPrunerStatus) MarkInstallerSetAvailable() {
	condSet.Manage(pruner).MarkTrue(InstallerSetAvailable)
}

func (pruner *TektonPrunerStatus) MarkInstallerSetReady() {
	condSet.Manage(pruner).MarkTrue(InstallerSetReady)
}

func (pruner *TektonPrunerStatus) MarkPostReconcilerComplete() {
	condSet.Manage(pruner).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (pruner *TektonPrunerStatus) MarkDependenciesInstalled() {
	condSet.Manage(pruner).MarkTrue(DependenciesInstalled)
}

func (pruner *TektonPrunerStatus) MarkNotReady(msg string) {
	condSet.Manage(pruner).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (pruner *TektonPrunerStatus) MarkPreReconcilerFailed(msg string) {
	pruner.MarkNotReady("PreReconciliation failed")
	condSet.Manage(pruner).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (pruner *TektonPrunerStatus) MarkInstallerSetNotAvailable(msg string) {
	pruner.MarkNotReady("TektonInstallerSet not ready")
	condSet.Manage(pruner).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (pruner *TektonPrunerStatus) MarkInstallerSetNotReady(msg string) {
	pruner.MarkNotReady("TektonInstallerSet not ready")
	condSet.Manage(pruner).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (pruner *TektonPrunerStatus) MarkPostReconcilerFailed(msg string) {
	pruner.MarkNotReady("PostReconciliation failed")
	condSet.Manage(pruner).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (pruner *TektonPrunerStatus) MarkDependencyInstalling(msg string) {
	pruner.MarkNotReady("Dependencies installing")
	condSet.Manage(pruner).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (pruner *TektonPrunerStatus) MarkDependencyMissing(msg string) {
	pruner.MarkNotReady("Missing Dependencies for TektonPruner")
	condSet.Manage(pruner).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (pruner *TektonPrunerStatus) GetTektonInstallerSet() string {
	return pruner.TektonInstallerSet
}

func (pruner *TektonPrunerStatus) SetTektonInstallerSet(installerSet string) {
	pruner.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (pruner *TektonPrunerStatus) GetVersion() string {
	return pruner.Version
}

// SetVersion sets the currently installed version of the component.
func (pruner *TektonPrunerStatus) SetVersion(version string) {
	pruner.Version = version
}
