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
	_ TektonComponentStatus = (*TektonSchedulerStatus)(nil)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonScheduler
func (Scheduler *TektonScheduler) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonScheduler)
}

func (Scheduler *TektonScheduler) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonScheduler)
}

// GetCondition returns the current condition of a given condition type
func (Scheduler *TektonSchedulerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return condSet.Manage(Scheduler).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonSchedulerStatus
func (Scheduler *TektonSchedulerStatus) InitializeConditions() {
	condSet.Manage(Scheduler).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (Scheduler *TektonSchedulerStatus) IsReady() bool {
	return condSet.Manage(Scheduler).IsHappy()
}

func (Scheduler *TektonSchedulerStatus) MarkPreReconcilerComplete() {
	condSet.Manage(Scheduler).MarkTrue(PreReconciler)
}

func (Scheduler *TektonSchedulerStatus) MarkInstallerSetAvailable() {
	condSet.Manage(Scheduler).MarkTrue(InstallerSetAvailable)
}

func (Scheduler *TektonSchedulerStatus) MarkInstallerSetReady() {
	condSet.Manage(Scheduler).MarkTrue(InstallerSetReady)
}

func (Scheduler *TektonSchedulerStatus) MarkPostReconcilerComplete() {
	condSet.Manage(Scheduler).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (Scheduler *TektonSchedulerStatus) MarkDependenciesInstalled() {
	condSet.Manage(Scheduler).MarkTrue(DependenciesInstalled)
}

func (Scheduler *TektonSchedulerStatus) MarkNotReady(msg string) {
	condSet.Manage(Scheduler).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (Scheduler *TektonSchedulerStatus) MarkPreReconcilerFailed(msg string) {
	Scheduler.MarkNotReady("PreReconciliation failed")
	condSet.Manage(Scheduler).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (Scheduler *TektonSchedulerStatus) MarkInstallerSetNotAvailable(msg string) {
	Scheduler.MarkNotReady("TektonScheduler not ready")
	condSet.Manage(Scheduler).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (Scheduler *TektonSchedulerStatus) MarkInstallerSetNotReady(msg string) {
	Scheduler.MarkNotReady("TektonScheduler not ready")
	condSet.Manage(Scheduler).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (Scheduler *TektonSchedulerStatus) MarkPostReconcilerFailed(msg string) {
	Scheduler.MarkNotReady("PostReconciliation failed")
	condSet.Manage(Scheduler).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (Scheduler *TektonSchedulerStatus) MarkDependencyInstalling(msg string) {
	Scheduler.MarkNotReady("Dependencies installing")
	condSet.Manage(Scheduler).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (Scheduler *TektonSchedulerStatus) MarkDependencyMissing(msg string) {
	Scheduler.MarkNotReady("Missing Dependencies for TektonScheduler")
	condSet.Manage(Scheduler).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (Scheduler *TektonSchedulerStatus) GetTektonScheduler() string {
	return Scheduler.TektonScheduler
}

func (Scheduler *TektonSchedulerStatus) SetTektonScheduler(installerSet string) {
	Scheduler.TektonScheduler = installerSet
}

// GetVersion gets the currently installed version of the component.
func (Scheduler *TektonSchedulerStatus) GetVersion() string {
	return Scheduler.Version
}

// SetVersion sets the currently installed version of the component.
func (Scheduler *TektonSchedulerStatus) SetVersion(version string) {
	Scheduler.Version = version
}
