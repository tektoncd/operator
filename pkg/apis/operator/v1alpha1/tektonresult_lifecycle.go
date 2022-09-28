/*
Copyright 2021 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    httr://www.apache.org/licenses/LICENSE-2.0

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
	_              TektonComponentStatus = (*TektonResultStatus)(nil)
	resultsCondSet                       = apis.NewLivingConditionSet(
		DependenciesInstalled,
		InstallerSetAvailable,
		InstallerSetReady,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonResult
func (tr *TektonResult) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonResult)
}

func (tr *TektonResult) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonResult)
}

// GetCondition returns the current condition of a given condition type
func (trs *TektonResultStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return resultsCondSet.Manage(trs).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonResultStatus
func (trs *TektonResultStatus) InitializeConditions() {
	resultsCondSet.Manage(trs).InitializeConditions()
}

// IsReady looks at the conditions returns true if they are all true.
func (trs *TektonResultStatus) IsReady() bool {
	return resultsCondSet.Manage(trs).IsHappy()
}

func (trs *TektonResultStatus) MarkNotReady(msg string) {
	resultsCondSet.Manage(trs).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (trs *TektonResultStatus) MarkInstallerSetAvailable() {
	resultsCondSet.Manage(trs).MarkTrue(InstallerSetAvailable)
}

func (trs *TektonResultStatus) MarkInstallerSetNotAvailable(msg string) {
	trs.MarkNotReady("TektonInstallerSet not ready")
	resultsCondSet.Manage(trs).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (trs *TektonResultStatus) MarkInstallerSetReady() {
	resultsCondSet.Manage(trs).MarkTrue(InstallerSetReady)
}

func (trs *TektonResultStatus) MarkInstallerSetNotReady(msg string) {
	trs.MarkNotReady("TektonInstallerSet not ready")
	resultsCondSet.Manage(trs).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (trs *TektonResultStatus) MarkDependenciesInstalled() {
	resultsCondSet.Manage(trs).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (trs *TektonResultStatus) MarkDependencyInstalling(msg string) {
	resultsCondSet.Manage(trs).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (trs *TektonResultStatus) MarkDependencyMissing(msg string) {
	resultsCondSet.Manage(trs).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}

func (trs *TektonResultStatus) GetTektonInstallerSet() string {
	return trs.TektonInstallerSet
}

func (trs *TektonResultStatus) SetTektonInstallerSet(installerSet string) {
	trs.TektonInstallerSet = installerSet
}

// GetVersion gets the currently installed version of the component.
func (trs *TektonResultStatus) GetVersion() string {
	return trs.Version
}

// SetVersion sets the currently installed version of the component.
func (trs *TektonResultStatus) SetVersion(version string) {
	trs.Version = version
}
