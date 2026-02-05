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
	_ TektonComponentStatus = (*TektonMulticlusterProxyAAEStatus)(nil)

	multiclusterProxyAAECondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonMulticlusterProxyAAE
func (t *TektonMulticlusterProxyAAE) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonMulticlusterProxyAAE)
}

// GetGroupVersionKind returns SchemeGroupVersion of a TektonMulticlusterProxyAAE
func (t *TektonMulticlusterProxyAAE) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonMulticlusterProxyAAE)
}

// GetCondition returns the current condition of a given condition type
func (t *TektonMulticlusterProxyAAEStatus) GetCondition(ct apis.ConditionType) *apis.Condition {
	return multiclusterProxyAAECondSet.Manage(t).GetCondition(ct)
}

// InitializeConditions initializes conditions of TektonMulticlusterProxyAAEStatus
func (t *TektonMulticlusterProxyAAEStatus) InitializeConditions() {
	multiclusterProxyAAECondSet.Manage(t).InitializeConditions()
}

// IsReady returns true if all conditions are satisfied
func (t *TektonMulticlusterProxyAAEStatus) IsReady() bool {
	return multiclusterProxyAAECondSet.Manage(t).IsHappy()
}

// MarkNotReady marks the component as not ready
func (t *TektonMulticlusterProxyAAEStatus) MarkNotReady(msg string) {
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

// MarkPreReconcilerComplete marks PreReconciler as complete
func (t *TektonMulticlusterProxyAAEStatus) MarkPreReconcilerComplete() {
	multiclusterProxyAAECondSet.Manage(t).MarkTrue(PreReconciler)
}

// MarkInstallerSetAvailable marks InstallerSetAvailable as true
func (t *TektonMulticlusterProxyAAEStatus) MarkInstallerSetAvailable() {
	multiclusterProxyAAECondSet.Manage(t).MarkTrue(InstallerSetAvailable)
}

// MarkInstallerSetReady marks InstallerSetReady as true
func (t *TektonMulticlusterProxyAAEStatus) MarkInstallerSetReady() {
	multiclusterProxyAAECondSet.Manage(t).MarkTrue(InstallerSetReady)
}

// MarkPostReconcilerComplete marks PostReconciler as complete
func (t *TektonMulticlusterProxyAAEStatus) MarkPostReconcilerComplete() {
	multiclusterProxyAAECondSet.Manage(t).MarkTrue(PostReconciler)
}

// MarkPreReconcilerFailed marks PreReconciler as failed
func (t *TektonMulticlusterProxyAAEStatus) MarkPreReconcilerFailed(msg string) {
	t.MarkNotReady("PreReconciliation failed")
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

// MarkInstallerSetNotAvailable marks InstallerSet as not available
func (t *TektonMulticlusterProxyAAEStatus) MarkInstallerSetNotAvailable(msg string) {
	t.MarkNotReady("TektonInstallerSet not ready")
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

// MarkInstallerSetNotReady marks InstallerSet as not ready
func (t *TektonMulticlusterProxyAAEStatus) MarkInstallerSetNotReady(msg string) {
	t.MarkNotReady("TektonInstallerSet not ready")
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

// MarkPostReconcilerFailed marks PostReconciler as failed
func (t *TektonMulticlusterProxyAAEStatus) MarkPostReconcilerFailed(msg string) {
	t.MarkNotReady("PostReconciliation failed")
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks DependenciesInstalled as false
func (t *TektonMulticlusterProxyAAEStatus) MarkDependencyInstalling(msg string) {
	t.MarkNotReady("Dependencies installing")
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks DependenciesInstalled as false
func (t *TektonMulticlusterProxyAAEStatus) MarkDependencyMissing(msg string) {
	t.MarkNotReady("Missing Dependencies for TektonMulticlusterProxyAAE")
	multiclusterProxyAAECondSet.Manage(t).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

// MarkDependenciesInstalled marks DependenciesInstalled as true
func (t *TektonMulticlusterProxyAAEStatus) MarkDependenciesInstalled() {
	multiclusterProxyAAECondSet.Manage(t).MarkTrue(DependenciesInstalled)
}

// GetVersion returns the installed version
func (t *TektonMulticlusterProxyAAEStatus) GetVersion() string {
	return t.Version
}

// SetVersion sets the installed version
func (t *TektonMulticlusterProxyAAEStatus) SetVersion(version string) {
	t.Version = version
}
