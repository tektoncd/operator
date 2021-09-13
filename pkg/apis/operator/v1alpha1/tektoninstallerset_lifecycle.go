/*
Copyright 2021 The Tekton Authors

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
	CrdInstalled         apis.ConditionType = "CrdsInstalled"
	ClustersScoped       apis.ConditionType = "ClusterScopedResourcesInstalled"
	NamespaceScoped      apis.ConditionType = "NamespaceScopedResourcesInstalled"
	DeploymentsAvailable apis.ConditionType = "DeploymentsAvailable"
	WebhookReady         apis.ConditionType = "WebhooksReady"
	ControllerReady      apis.ConditionType = "ControllersReady"
	AllDeploymentsReady  apis.ConditionType = "AllDeploymentsReady"
)

var (
	installerSetCondSet = apis.NewLivingConditionSet(
		CrdInstalled,
		ClustersScoped,
		NamespaceScoped,
		DeploymentsAvailable,
		WebhookReady,
		ControllerReady,
		AllDeploymentsReady,
	)
)

func (tis *TektonInstallerSet) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonInstallerSet)
}

func (tis *TektonInstallerSetStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return installerSetCondSet.Manage(tis).GetCondition(t)
}

func (tis *TektonInstallerSetStatus) InitializeConditions() {
	installerSetCondSet.Manage(tis).InitializeConditions()
}

func (tis *TektonInstallerSetStatus) IsReady() bool {
	return installerSetCondSet.Manage(tis).IsHappy()
}

func (tis *TektonInstallerSetStatus) MarkReady() {
	installerSetCondSet.Manage(tis).MarkTrue(apis.ConditionReady)
}

func (tis *TektonInstallerSetStatus) MarkCRDsInstalled() {
	installerSetCondSet.Manage(tis).MarkTrue(CrdInstalled)
}

func (tis *TektonInstallerSetStatus) MarkClustersScopedResourcesInstalled() {
	installerSetCondSet.Manage(tis).MarkTrue(ClustersScoped)
}

func (tis *TektonInstallerSetStatus) MarkNamespaceScopedResourcesInstalled() {
	installerSetCondSet.Manage(tis).MarkTrue(NamespaceScoped)
}

func (tis *TektonInstallerSetStatus) MarkDeploymentsAvailable() {
	installerSetCondSet.Manage(tis).MarkTrue(DeploymentsAvailable)
}

func (tis *TektonInstallerSetStatus) MarkWebhookReady() {
	installerSetCondSet.Manage(tis).MarkTrue(WebhookReady)
}

func (tis *TektonInstallerSetStatus) MarkControllerReady() {
	installerSetCondSet.Manage(tis).MarkTrue(ControllerReady)
}

func (tis *TektonInstallerSetStatus) MarkAllDeploymentsReady() {
	installerSetCondSet.Manage(tis).MarkTrue(AllDeploymentsReady)
}

func (tis *TektonInstallerSetStatus) MarkNotReady(msg string) {
	installerSetCondSet.Manage(tis).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkCRDsInstallationFailed(msg string) {
	tis.MarkNotReady("CRDs installation failed")
	installerSetCondSet.Manage(tis).MarkFalse(
		CrdInstalled,
		"Error",
		"Install failed with message: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkClustersScopedInstallationFailed(msg string) {
	tis.MarkNotReady("Cluster Scoped resources installation failed")
	installerSetCondSet.Manage(tis).MarkFalse(
		ClustersScoped,
		"Error",
		"Install failed with message: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkNamespaceScopedInstallationFailed(msg string) {
	tis.MarkNotReady("Namespace Scoped resources installation failed")
	installerSetCondSet.Manage(tis).MarkFalse(
		NamespaceScoped,
		"Error",
		"Install failed with message: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkDeploymentsAvailableFailed(msg string) {
	tis.MarkNotReady("Deployment resources installation failed")
	installerSetCondSet.Manage(tis).MarkFalse(
		DeploymentsAvailable,
		"Error",
		"Install failed with message: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkWebhookNotReady(msg string) {
	tis.MarkNotReady("Webhooks not available")
	installerSetCondSet.Manage(tis).MarkFalse(
		WebhookReady,
		"Error",
		"Webhook: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkControllerNotReady(msg string) {
	tis.MarkNotReady("Controller Deployment not available")
	installerSetCondSet.Manage(tis).MarkFalse(
		ControllerReady,
		"Error",
		"Controller: %s", msg)
}

func (tis *TektonInstallerSetStatus) MarkAllDeploymentsNotReady(msg string) {
	tis.MarkNotReady("All Deployments not available")
	installerSetCondSet.Manage(tis).MarkFalse(
		AllDeploymentsReady,
		"Error",
		"Deployment: %s", msg)
}
