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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

const (
	PreInstall      apis.ConditionType = "PreInstall"
	ComponentsReady apis.ConditionType = "ComponentsReady"
	PostInstall     apis.ConditionType = "PostInstall"
	PreUpgrade      apis.ConditionType = "PreUpgrade"
	PostUpgrade     apis.ConditionType = "PostUpgrade"
)

var (
	configCondSet = apis.NewLivingConditionSet(
		PreInstall,
		ComponentsReady,
		PostInstall,
		PreUpgrade,
		PostUpgrade,
	)
)

func (tc *TektonConfig) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonConfig)
}

func (tc *TektonConfig) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonConfig)
}

func (tcs *TektonConfigStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return configCondSet.Manage(tcs).GetCondition(t)
}

func (tcs *TektonConfigStatus) InitializeConditions() {
	configCondSet.Manage(tcs).InitializeConditions()
}

func (tcs *TektonConfigStatus) IsReady() bool {
	return configCondSet.Manage(tcs).IsHappy()
}

func (tcs *TektonConfigStatus) MarkPreInstallComplete() {
	configCondSet.Manage(tcs).MarkTrue(PreInstall)
}

func (tcs *TektonConfigStatus) MarkComponentsReady() {
	configCondSet.Manage(tcs).MarkTrue(ComponentsReady)
}

func (tcs *TektonConfigStatus) MarkPostInstallComplete() {
	configCondSet.Manage(tcs).MarkTrue(PostInstall)
}

func (tcs *TektonConfigStatus) MarkNotReady(msg string) {
	configCondSet.Manage(tcs).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tcs *TektonConfigStatus) MarkPreInstallFailed(msg string) {
	tcs.MarkNotReady("PreReconciliation failed")
	configCondSet.Manage(tcs).MarkFalse(
		PreInstall,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tcs *TektonConfigStatus) MarkComponentNotReady(msg string) {
	tcs.MarkNotReady("Components not ready")
	configCondSet.Manage(tcs).MarkFalse(
		ComponentsReady,
		"Error",
		"Components not in ready state: %s", msg)
}

func (tcs *TektonConfigStatus) MarkPostInstallFailed(msg string) {
	tcs.MarkNotReady("PostReconciliation failed")
	configCondSet.Manage(tcs).MarkFalse(
		PostInstall,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

func (tcs *TektonConfigStatus) MarkPreUpgradeComplete() bool {
	condition := configCondSet.Manage(tcs).GetCondition(PreUpgrade)
	if condition != nil && condition.Status == corev1.ConditionTrue {
		return false
	}
	configCondSet.Manage(tcs).MarkTrue(PreUpgrade)
	return true
}

func (tcs *TektonConfigStatus) MarkPostUpgradeComplete() bool {
	condition := configCondSet.Manage(tcs).GetCondition(PostUpgrade)
	if condition != nil && condition.Status == corev1.ConditionTrue {
		return false
	}
	configCondSet.Manage(tcs).MarkTrue(PostUpgrade)
	return true
}

func (tcs *TektonConfigStatus) MarkPreUpgradeFalse(reason, msg string) bool {
	condition := configCondSet.Manage(tcs).GetCondition(PreUpgrade)
	if condition != nil && condition.Status == corev1.ConditionFalse && condition.Reason == reason && condition.Message == msg {
		return false
	}
	configCondSet.Manage(tcs).MarkFalse(PreUpgrade, reason, "%s", msg)
	return true
}

func (tcs *TektonConfigStatus) MarkPostUpgradeFalse(reason, msg string) bool {
	condition := configCondSet.Manage(tcs).GetCondition(PostUpgrade)
	if condition != nil && condition.Status == corev1.ConditionFalse && condition.Reason == reason && condition.Message == msg {
		return false
	}
	configCondSet.Manage(tcs).MarkFalse(PostUpgrade, reason, "%s", msg)
	return true
}

// GetVersion gets the currently installed version of the component.
func (tcs *TektonConfigStatus) GetVersion() string {
	return tcs.Version
}

// SetVersion sets the currently installed version of the component.
func (tcs *TektonConfigStatus) SetVersion(version string) {
	tcs.Version = version
}

// returns pre upgrade version
func (tcs *TektonConfigStatus) GetPreUpgradeVersion() string {
	if tcs.Annotations == nil {
		return ""
	}
	return tcs.Annotations[PreUpgradeVersionKey]
}

// updates pre upgrade version
func (tcs *TektonConfigStatus) SetPreUpgradeVersion(appliedUpgradeVersion string) {
	if tcs.Annotations == nil {
		tcs.Annotations = map[string]string{}
	}
	tcs.Annotations[PreUpgradeVersionKey] = appliedUpgradeVersion
}

// returns post upgrade version
func (tcs *TektonConfigStatus) GetPostUpgradeVersion() string {
	if tcs.Annotations == nil {
		return ""
	}
	return tcs.Annotations[PostUpgradeVersionKey]
}

// updates post upgrade version
func (tcs *TektonConfigStatus) SetPostUpgradeVersion(appliedUpgradeVersion string) {
	if tcs.Annotations == nil {
		tcs.Annotations = map[string]string{}
	}
	tcs.Annotations[PostUpgradeVersionKey] = appliedUpgradeVersion
}
