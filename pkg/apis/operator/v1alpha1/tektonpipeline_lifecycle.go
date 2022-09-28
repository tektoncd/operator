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
	PreReconciler         apis.ConditionType = "PreReconciler"
	InstallerSetAvailable apis.ConditionType = "InstallerSetAvailable"
	InstallerSetReady     apis.ConditionType = "InstallerSetReady"
	PostReconciler        apis.ConditionType = "PostReconciler"
)

var (
	// TODO: Add this back after refactoring all components
	// and updating TektonComponentStatus to have updated
	// conditions
	//_ TektonComponentStatus = (*TektonPipelineStatus)(nil)

	pipelineCondSet = apis.NewLivingConditionSet(
		PreReconciler,
		InstallerSetAvailable,
		InstallerSetReady,
		PostReconciler,
	)
)

func (tp *TektonPipeline) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonPipeline)
}

func (tp *TektonPipeline) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonPipeline)
}

func (tps *TektonPipelineStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pipelineCondSet.Manage(tps).GetCondition(t)
}

func (tps *TektonPipelineStatus) InitializeConditions() {
	pipelineCondSet.Manage(tps).InitializeConditions()
}

func (tps *TektonPipelineStatus) IsReady() bool {
	return pipelineCondSet.Manage(tps).IsHappy()
}

func (tps *TektonPipelineStatus) MarkPreReconcilerComplete() {
	pipelineCondSet.Manage(tps).MarkTrue(PreReconciler)
}

func (tps *TektonPipelineStatus) MarkInstallerSetAvailable() {
	pipelineCondSet.Manage(tps).MarkTrue(InstallerSetAvailable)
}

func (tps *TektonPipelineStatus) MarkInstallerSetReady() {
	pipelineCondSet.Manage(tps).MarkTrue(InstallerSetReady)
}

func (tps *TektonPipelineStatus) MarkPostReconcilerComplete() {
	pipelineCondSet.Manage(tps).MarkTrue(PostReconciler)
}

func (tps *TektonPipelineStatus) MarkNotReady(msg string) {
	pipelineCondSet.Manage(tps).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tps *TektonPipelineStatus) MarkPreReconcilerFailed(msg string) {
	tps.MarkNotReady("PreReconciliation failed")
	pipelineCondSet.Manage(tps).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (tps *TektonPipelineStatus) MarkInstallerSetNotAvailable(msg string) {
	tps.MarkNotReady("TektonInstallerSet not ready")
	pipelineCondSet.Manage(tps).MarkFalse(
		InstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tps *TektonPipelineStatus) MarkInstallerSetNotReady(msg string) {
	tps.MarkNotReady("TektonInstallerSet not ready")
	pipelineCondSet.Manage(tps).MarkFalse(
		InstallerSetReady,
		"Error",
		"Installer set not ready: %s", msg)
}

func (tps *TektonPipelineStatus) MarkPostReconcilerFailed(msg string) {
	tps.MarkNotReady("PostReconciliation failed")
	pipelineCondSet.Manage(tps).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

// TODO: below methods are not required for TektonPipeline
// but as extension implements TektonComponent we need to defined them
// this will be removed

func (tps *TektonPipelineStatus) GetTektonInstallerSet() string {
	return tps.TektonInstallerSet
}

func (tps *TektonPipelineStatus) SetTektonInstallerSet(installerSet string) {
	tps.TektonInstallerSet = installerSet
}

func (tps *TektonPipelineStatus) GetVersion() string {
	return tps.Version
}

func (tps *TektonPipelineStatus) SetVersion(version string) {
	tps.Version = version
}
