/*
Copyright 2020 The Tekton Authors

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

const (
	DependenciesInstalled apis.ConditionType = "DependenciesInstalled"
)

var (
	// TODO: Add this back after refactoring all components
	// and updating TektonComponentStatus to have updated
	// conditions
	//_ TektonComponentStatus = (*TektonTriggerStatus)(nil)

	triggersCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

func (tr *TektonTrigger) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonTrigger)
}

func (tr *TektonTrigger) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonTrigger)
}

func (tts *TektonTriggerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return triggersCondSet.Manage(tts).GetCondition(t)
}

func (tts *TektonTriggerStatus) InitializeConditions() {
	triggersCondSet.Manage(tts).InitializeConditions()
}

func (tts *TektonTriggerStatus) IsReady() bool {
	return triggersCondSet.Manage(tts).IsHappy()
}

func (tts *TektonTriggerStatus) IsNewInstallation() bool {
	return tts.Status.GetCondition(apis.ConditionReady).IsUnknown()
}

func (tts *TektonTriggerStatus) MarkPreReconcilerComplete() {
	triggersCondSet.Manage(tts).MarkTrue(PreReconciler)
}

func (tts *TektonTriggerStatus) MarkInstallerSetAvailable() {
	triggersCondSet.Manage(tts).MarkTrue(InstallerSetAvailable)
}

func (tts *TektonTriggerStatus) MarkInstallerSetReady() {
	triggersCondSet.Manage(tts).MarkTrue(InstallerSetReady)
}

func (tts *TektonTriggerStatus) MarkPostReconcilerComplete() {
	triggersCondSet.Manage(tts).MarkTrue(PostReconciler)
}

func (tts *TektonTriggerStatus) MarkDependenciesInstalled() {
	triggersCondSet.Manage(tts).MarkTrue(DependenciesInstalled)
}

func (tts *TektonTriggerStatus) MarkNotReady(msg string) {
	triggersCondSet.Manage(tts).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tts *TektonTriggerStatus) MarkPreReconcilerFailed(msg string) {
	tts.MarkNotReady("PreReconciliation failed")
	triggersCondSet.Manage(tts).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tts *TektonTriggerStatus) MarkInstallerSetNotAvailable(msg string) {
	tts.MarkNotReady("TektonInstallerSet not ready")
	triggersCondSet.Manage(tts).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tts *TektonTriggerStatus) MarkInstallerSetNotReady(msg string) {
	tts.MarkNotReady("TektonInstallerSet not ready")
	triggersCondSet.Manage(tts).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tts *TektonTriggerStatus) MarkPostReconcilerFailed(msg string) {
	tts.MarkNotReady("PostReconciliation failed")
	triggersCondSet.Manage(tts).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

func (tts *TektonTriggerStatus) MarkDependencyInstalling(msg string) {
	tts.MarkNotReady("Dependencies installing")
	triggersCondSet.Manage(tts).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependencies are installing: %s", msg)
}

func (tts *TektonTriggerStatus) MarkDependencyMissing(msg string) {
	tts.MarkNotReady("Missing Dependencies for TektonTriggers")
	triggersCondSet.Manage(tts).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependencies are missing: %s", msg)
}

func (tts *TektonTriggerStatus) GetVersion() string {
	return tts.Version
}

func (tts *TektonTriggerStatus) SetVersion(version string) {
	tts.Version = version
}
