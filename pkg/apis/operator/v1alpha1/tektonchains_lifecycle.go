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
	_ TektonComponentStatus = (*TektonChainsStatus)(nil)

	chainsCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonChains
func (tc *TektonChains) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonChains)
}

func (tc *TektonChains) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonChains)
}

// GetCondition returns the current condition of a given condition type
func (tcs *TektonChainsStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return chainsCondSet.Manage(tcs).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonChainsStatus
func (tcs *TektonChainsStatus) InitializeConditions() {
	chainsCondSet.Manage(tcs).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (tcs *TektonChainsStatus) IsReady() bool {
	return chainsCondSet.Manage(tcs).IsHappy()
}

func (tcs *TektonChainsStatus) MarkPreReconcilerComplete() {
	chainsCondSet.Manage(tcs).MarkTrue(PreReconciler)
}

func (tcs *TektonChainsStatus) MarkInstallerSetAvailable() {
	chainsCondSet.Manage(tcs).MarkTrue(InstallerSetAvailable)
}

func (tcs *TektonChainsStatus) MarkInstallerSetReady() {
	chainsCondSet.Manage(tcs).MarkTrue(InstallerSetReady)
}

func (tcs *TektonChainsStatus) MarkPostReconcilerComplete() {
	chainsCondSet.Manage(tcs).MarkTrue(PostReconciler)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (tcs *TektonChainsStatus) MarkDependenciesInstalled() {
	chainsCondSet.Manage(tcs).MarkTrue(DependenciesInstalled)
}

func (tcs *TektonChainsStatus) MarkNotReady(msg string) {
	chainsCondSet.Manage(tcs).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tcs *TektonChainsStatus) MarkPreReconcilerFailed(msg string) {
	tcs.MarkNotReady("PreReconciliation failed")
	chainsCondSet.Manage(tcs).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tcs *TektonChainsStatus) MarkInstallerSetNotAvailable(msg string) {
	tcs.MarkNotReady("TektonInstallerSet not ready")
	chainsCondSet.Manage(tcs).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tcs *TektonChainsStatus) MarkInstallerSetNotReady(msg string) {
	tcs.MarkNotReady("TektonInstallerSet not ready")
	chainsCondSet.Manage(tcs).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tcs *TektonChainsStatus) MarkPostReconcilerFailed(msg string) {
	tcs.MarkNotReady("PostReconciliation failed")
	chainsCondSet.Manage(tcs).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (tcs *TektonChainsStatus) MarkDependencyInstalling(msg string) {
	tcs.MarkNotReady("Dependencies installing")
	chainsCondSet.Manage(tcs).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (tcs *TektonChainsStatus) MarkDependencyMissing(msg string) {
	tcs.MarkNotReady("Missing Dependencies for TektonChains")
	chainsCondSet.Manage(tcs).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (tcs *TektonChainsStatus) GetTektonInstallerSet() string {
	return tcs.TektonInstallerSet
}

func (tcs *TektonChainsStatus) SetTektonInstallerSet(installerSet string) {
	tcs.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (tcs *TektonChainsStatus) GetVersion() string {
	return tcs.Version
}

// SetVersion sets the currently installed version of the component.
func (tcs *TektonChainsStatus) SetVersion(version string) {
	tcs.Version = version
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (tcs *TektonChainsStatus) MarkInstallSucceeded() {
	panic("implement me")
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (tcs *TektonChainsStatus) MarkInstallFailed(msg string) {
	panic("implement me")
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (tcs *TektonChainsStatus) MarkDeploymentsAvailable() {
	panic("implement me")
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (tcs *TektonChainsStatus) MarkDeploymentsNotReady() {
	panic("implement me")
}

// GetManifests gets the url links of the manifests.
func (tcs *TektonChainsStatus) GetManifests() []string {
	panic("implement me")
}
