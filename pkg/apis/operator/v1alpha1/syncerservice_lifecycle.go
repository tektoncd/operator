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
	_                    TektonComponentStatus = (*SyncerServiceStatus)(nil)
	syncerServiceCondSet                       = apis.NewLivingConditionSet(
		PreReconciler,
		DependenciesInstalled,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a SyncerService
func (ss *SyncerService) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindSyncerService)
}

func (ss *SyncerService) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindSyncerService)
}

// GetCondition returns the current condition of a given condition type
func (sss *SyncerServiceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return syncerServiceCondSet.Manage(sss).GetCondition(t)
}

// InitializeConditions initializes conditions of an SyncerServiceStatus
func (sss *SyncerServiceStatus) InitializeConditions() {
	syncerServiceCondSet.Manage(sss).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (sss *SyncerServiceStatus) IsReady() bool {
	return syncerServiceCondSet.Manage(sss).IsHappy()
}

func (sss *SyncerServiceStatus) MarkNotReady(msg string) {
	syncerServiceCondSet.Manage(sss).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (sss *SyncerServiceStatus) MarkPreReconcilerComplete() {
	syncerServiceCondSet.Manage(sss).MarkTrue(PreReconciler)
}

func (sss *SyncerServiceStatus) MarkInstallerSetAvailable() {
	syncerServiceCondSet.Manage(sss).MarkTrue(InstallerSetAvailable)
}

func (sss *SyncerServiceStatus) MarkInstallerSetReady() {
	syncerServiceCondSet.Manage(sss).MarkTrue(InstallerSetReady)
}

func (sss *SyncerServiceStatus) MarkInstallerSetNotAvailable(msg string) {
	sss.MarkNotReady("InstallerSet not ready")
	syncerServiceCondSet.Manage(sss).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (sss *SyncerServiceStatus) MarkInstallerSetNotReady(msg string) {
	sss.MarkNotReady("InstallerSet not ready")
	syncerServiceCondSet.Manage(sss).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (sss *SyncerServiceStatus) MarkPostReconcilerComplete() {
	syncerServiceCondSet.Manage(sss).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (sss *SyncerServiceStatus) MarkDependenciesInstalled() {
	syncerServiceCondSet.Manage(sss).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false
func (sss *SyncerServiceStatus) MarkDependencyInstalling(msg string) {
	syncerServiceCondSet.Manage(sss).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false
func (sss *SyncerServiceStatus) MarkDependencyMissing(msg string) {
	syncerServiceCondSet.Manage(sss).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (sss *SyncerServiceStatus) GetSyncerServiceInstallerSet() string {
	return sss.SyncerServiceInstallerSet
}

func (sss *SyncerServiceStatus) SetSyncerServiceInstallerSet(installerSet string) {
	sss.SyncerServiceInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (sss *SyncerServiceStatus) GetVersion() string {
	return sss.Version
}

// SetVersion sets the currently installed version of the component.
func (sss *SyncerServiceStatus) SetVersion(version string) {
	sss.Version = version
}
