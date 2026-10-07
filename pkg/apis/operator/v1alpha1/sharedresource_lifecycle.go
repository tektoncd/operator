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
	_ TektonComponentStatus = (*SharedResourceStatus)(nil)

	SharedResourceCondSet = apis.NewLivingConditionSet(
		PreReconciler,
		DependenciesInstalled,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of an SharedResource
func (sr *SharedResource) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindSharedResource)
}

func (sr *SharedResource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindSharedResource)
}

// GetCondition returns the current condition of a given condition type
func (status *SharedResourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return SharedResourceCondSet.Manage(status).GetCondition(t)
}

// InitializeConditions initializes conditions of an SharedResourceStatus
func (status *SharedResourceStatus) InitializeConditions() {
	SharedResourceCondSet.Manage(status).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (status *SharedResourceStatus) IsReady() bool {
	return SharedResourceCondSet.Manage(status).IsHappy()
}

func (status *SharedResourceStatus) MarkNotReady(msg string) {
	SharedResourceCondSet.Manage(status).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (status *SharedResourceStatus) MarkPreReconcilerComplete() {
	SharedResourceCondSet.Manage(status).MarkTrue(PreReconciler)
}

func (status *SharedResourceStatus) MarkInstallerSetAvailable() {
	SharedResourceCondSet.Manage(status).MarkTrue(InstallerSetAvailable)
}

func (status *SharedResourceStatus) MarkInstallerSetReady() {
	SharedResourceCondSet.Manage(status).MarkTrue(InstallerSetReady)
}

func (status *SharedResourceStatus) MarkInstallerSetNotAvailable(msg string) {
	status.MarkNotReady("InstallerSet not ready")
	SharedResourceCondSet.Manage(status).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (status *SharedResourceStatus) MarkInstallerSetNotReady(msg string) {
	status.MarkNotReady("InstallerSet not ready")
	SharedResourceCondSet.Manage(status).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (status *SharedResourceStatus) MarkPostReconcilerComplete() {
	SharedResourceCondSet.Manage(status).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (status *SharedResourceStatus) MarkDependenciesInstalled() {
	SharedResourceCondSet.Manage(status).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false
func (status *SharedResourceStatus) MarkDependencyInstalling(msg string) {
	SharedResourceCondSet.Manage(status).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false
func (status *SharedResourceStatus) MarkDependencyMissing(msg string) {
	SharedResourceCondSet.Manage(status).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (status *SharedResourceStatus) MarkPreReconcilerFailed(msg string) {
	status.MarkNotReady("PreReconciliation failed")
	SharedResourceCondSet.Manage(status).MarkFalse(
		PreReconciler,
		"Error",
		msg,
	)
}

func (status *SharedResourceStatus) MarkPostReconcilerFailed(msg string) {
	status.MarkNotReady("PostReconciliation failed")
	SharedResourceCondSet.Manage(status).MarkFalse(
		PostReconciler,
		"Error",
		msg,
	)
}

func (status *SharedResourceStatus) GetTektonInstallerSet() string {
	return status.TektonInstallerSet
}

func (status *SharedResourceStatus) SetTektonInstallerSet(tektonInstallerSet string) {
	status.TektonInstallerSet = tektonInstallerSet
}

// GetVersion gets the currently installed version of the component.
func (status *SharedResourceStatus) GetVersion() string {
	return status.Version
}

// SetVersion sets the currently installed version of the component.
func (status *SharedResourceStatus) SetVersion(version string) {
	status.Version = version
}
