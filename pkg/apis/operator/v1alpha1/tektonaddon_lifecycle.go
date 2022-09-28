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

var (
	addonsCondSet = apis.NewLivingConditionSet(
		DependenciesInstalled,
		PreReconciler,
		InstallerSetReady,
		PostReconciler,
	)
)

func (tp *TektonAddon) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonAddon)
}

func (tp *TektonAddon) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonAddon)
}

func (tas *TektonAddonStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return addonsCondSet.Manage(tas).GetCondition(t)
}

func (tas *TektonAddonStatus) InitializeConditions() {
	addonsCondSet.Manage(tas).InitializeConditions()
}

func (tas *TektonAddonStatus) IsReady() bool {
	return addonsCondSet.Manage(tas).IsHappy()
}

func (tas *TektonAddonStatus) MarkPreReconcilerComplete() {
	addonsCondSet.Manage(tas).MarkTrue(PreReconciler)
}

func (tas *TektonAddonStatus) MarkInstallerSetReady() {
	addonsCondSet.Manage(tas).MarkTrue(InstallerSetReady)
}

func (tas *TektonAddonStatus) MarkPostReconcilerComplete() {
	addonsCondSet.Manage(tas).MarkTrue(PostReconciler)
}

func (tas *TektonAddonStatus) MarkDependenciesInstalled() {
	addonsCondSet.Manage(tas).MarkTrue(DependenciesInstalled)
}

func (tas *TektonAddonStatus) MarkNotReady(msg string) {
	addonsCondSet.Manage(tas).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tas *TektonAddonStatus) MarkPreReconcilerFailed(msg string) {
	tas.MarkNotReady("PreReconciliation failed")
	addonsCondSet.Manage(tas).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tas *TektonAddonStatus) MarkInstallerSetNotReady(msg string) {
	tas.MarkNotReady("TektonInstallerSet not ready")
	addonsCondSet.Manage(tas).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tas *TektonAddonStatus) MarkPostReconcilerFailed(msg string) {
	tas.MarkNotReady("PostReconciliation failed")
	addonsCondSet.Manage(tas).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

func (tas *TektonAddonStatus) MarkDependencyInstalling(msg string) {
	tas.MarkNotReady("Dependencies installing")
	addonsCondSet.Manage(tas).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependencies are installing: %s", msg)
}

func (tas *TektonAddonStatus) MarkDependencyMissing(msg string) {
	tas.MarkNotReady("Missing Dependencies for TektonTriggers")
	addonsCondSet.Manage(tas).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependencies are missing: %s", msg)
}

func (tas *TektonAddonStatus) GetVersion() string {
	return tas.Version
}

func (tas *TektonAddonStatus) SetVersion(version string) {
	tas.Version = version
}
