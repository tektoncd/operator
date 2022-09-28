/*
Copyright 2022 The Tekton Authors

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
	_ TektonComponentStatus = (*TektonChainStatus)(nil)

	chainCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonChain
func (tc *TektonChain) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonChain)
}

func (tc *TektonChain) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonChain)
}

// GetCondition returns the current condition of a given condition type
func (tcs *TektonChainStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return chainCondSet.Manage(tcs).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonChainStatus
func (tcs *TektonChainStatus) InitializeConditions() {
	chainCondSet.Manage(tcs).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (tcs *TektonChainStatus) IsReady() bool {
	return chainCondSet.Manage(tcs).IsHappy()
}

func (tcs *TektonChainStatus) MarkPreReconcilerComplete() {
	chainCondSet.Manage(tcs).MarkTrue(PreReconciler)
}

func (tcs *TektonChainStatus) MarkInstallerSetAvailable() {
	chainCondSet.Manage(tcs).MarkTrue(InstallerSetAvailable)
}

func (tcs *TektonChainStatus) MarkInstallerSetReady() {
	chainCondSet.Manage(tcs).MarkTrue(InstallerSetReady)
}

func (tcs *TektonChainStatus) MarkPostReconcilerComplete() {
	chainCondSet.Manage(tcs).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (tcs *TektonChainStatus) MarkDependenciesInstalled() {
	chainCondSet.Manage(tcs).MarkTrue(DependenciesInstalled)
}

func (tcs *TektonChainStatus) MarkNotReady(msg string) {
	chainCondSet.Manage(tcs).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tcs *TektonChainStatus) MarkPreReconcilerFailed(msg string) {
	tcs.MarkNotReady("PreReconciliation failed")
	chainCondSet.Manage(tcs).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tcs *TektonChainStatus) MarkInstallerSetNotAvailable(msg string) {
	tcs.MarkNotReady("TektonInstallerSet not ready")
	chainCondSet.Manage(tcs).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tcs *TektonChainStatus) MarkInstallerSetNotReady(msg string) {
	tcs.MarkNotReady("TektonInstallerSet not ready")
	chainCondSet.Manage(tcs).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tcs *TektonChainStatus) MarkPostReconcilerFailed(msg string) {
	tcs.MarkNotReady("PostReconciliation failed")
	chainCondSet.Manage(tcs).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (tcs *TektonChainStatus) MarkDependencyInstalling(msg string) {
	tcs.MarkNotReady("Dependencies installing")
	chainCondSet.Manage(tcs).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (tcs *TektonChainStatus) MarkDependencyMissing(msg string) {
	tcs.MarkNotReady("Missing Dependencies for TektonChain")
	chainCondSet.Manage(tcs).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (tcs *TektonChainStatus) GetTektonInstallerSet() string {
	return tcs.TektonInstallerSet
}

func (tcs *TektonChainStatus) SetTektonInstallerSet(installerSet string) {
	tcs.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (tcs *TektonChainStatus) GetVersion() string {
	return tcs.Version
}

// SetVersion sets the currently installed version of the component.
func (tcs *TektonChainStatus) SetVersion(version string) {
	tcs.Version = version
}
