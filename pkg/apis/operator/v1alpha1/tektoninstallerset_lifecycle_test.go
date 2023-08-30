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
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	apistest "knative.dev/pkg/apis/testing"
)

func TestTektonInstallerSetGroupVersionKind(t *testing.T) {
	r := &TektonInstallerSet{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonInstallerSet,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonInstallerSetHappyPath(t *testing.T) {
	tis := &TektonInstallerSetStatus{}
	tis.InitializeConditions()

	apistest.CheckConditionOngoing(tis, CrdInstalled, t)
	apistest.CheckConditionOngoing(tis, ClustersScoped, t)
	apistest.CheckConditionOngoing(tis, NamespaceScoped, t)
	apistest.CheckConditionOngoing(tis, DeploymentsAvailable, t)
	apistest.CheckConditionOngoing(tis, StatefulSetReady, t)
	apistest.CheckConditionOngoing(tis, WebhookReady, t)
	apistest.CheckConditionOngoing(tis, ControllerReady, t)
	apistest.CheckConditionOngoing(tis, AllDeploymentsReady, t)

	// Install succeeds.
	tis.MarkCRDsInstalled()
	apistest.CheckConditionSucceeded(tis, CrdInstalled, t)

	tis.MarkClustersScopedResourcesInstalled()
	apistest.CheckConditionSucceeded(tis, ClustersScoped, t)

	tis.MarkNamespaceScopedResourcesInstalled()
	apistest.CheckConditionSucceeded(tis, NamespaceScoped, t)

	tis.MarkDeploymentsAvailable()
	apistest.CheckConditionSucceeded(tis, DeploymentsAvailable, t)

	tis.MarkStatefulSetReady()
	apistest.CheckConditionSucceeded(tis, StatefulSetReady, t)

	// Initially Webhook will not be available
	tis.MarkWebhookNotReady("waiting for pods")
	apistest.CheckConditionFailed(tis, WebhookReady, t)

	tis.MarkWebhookReady()
	apistest.CheckConditionSucceeded(tis, WebhookReady, t)

	tis.MarkControllerReady()
	apistest.CheckConditionSucceeded(tis, ControllerReady, t)

	tis.MarkAllDeploymentsReady()
	apistest.CheckConditionSucceeded(tis, AllDeploymentsReady, t)

	if ready := tis.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestTektonInstallerSetErrorPath(t *testing.T) {
	tis := &TektonInstallerSetStatus{}
	tis.InitializeConditions()

	apistest.CheckConditionOngoing(tis, CrdInstalled, t)
	apistest.CheckConditionOngoing(tis, ClustersScoped, t)
	apistest.CheckConditionOngoing(tis, NamespaceScoped, t)
	apistest.CheckConditionOngoing(tis, DeploymentsAvailable, t)
	apistest.CheckConditionOngoing(tis, StatefulSetReady, t)
	apistest.CheckConditionOngoing(tis, WebhookReady, t)
	apistest.CheckConditionOngoing(tis, ControllerReady, t)
	apistest.CheckConditionOngoing(tis, AllDeploymentsReady, t)

	// CrdsInstall succeeds
	tis.MarkCRDsInstalled()
	apistest.CheckConditionSucceeded(tis, CrdInstalled, t)

	// ClustersScopedResources Install succeeds
	tis.MarkClustersScopedResourcesInstalled()
	apistest.CheckConditionSucceeded(tis, ClustersScoped, t)

	// NamespaceScopedResources Install succeeds
	tis.MarkNamespaceScopedResourcesInstalled()
	apistest.CheckConditionSucceeded(tis, NamespaceScoped, t)

	// DeploymentsAvailable succeeds
	tis.MarkDeploymentsAvailable()
	apistest.CheckConditionSucceeded(tis, DeploymentsAvailable, t)

	tis.MarkStatefulSetReady()
	apistest.CheckConditionSucceeded(tis, StatefulSetReady, t)

	// Initially Webhook will not be available
	tis.MarkWebhookNotReady("waiting for pods")
	apistest.CheckConditionFailed(tis, WebhookReady, t)

	tis.MarkWebhookReady()
	apistest.CheckConditionSucceeded(tis, WebhookReady, t)

	tis.MarkControllerReady()
	apistest.CheckConditionSucceeded(tis, ControllerReady, t)

	tis.MarkAllDeploymentsReady()
	apistest.CheckConditionSucceeded(tis, AllDeploymentsReady, t)

	if ready := tis.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}

	// Now in further reconciliation some error occurred in any of the
	// condition

	tis.MarkCRDsInstallationFailed("failed due to some error")
	apistest.CheckConditionFailed(tis, CrdInstalled, t)

	if ready := tis.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}
}
