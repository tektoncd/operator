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
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	apistest "knative.dev/pkg/apis/testing"
)

func TestOpenShiftPipelinesAsCodeGroupVersionKind(t *testing.T) {
	r := &OpenShiftPipelinesAsCode{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindOpenShiftPipelinesAsCode,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestOpenShiftPipelinesAsCodeHappyPath(t *testing.T) {
	pac := &OpenShiftPipelinesAsCodeStatus{}
	pac.InitializeConditions()

	apistest.CheckConditionOngoing(pac, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(pac, PreReconciler, t)
	apistest.CheckConditionOngoing(pac, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(pac, InstallerSetReady, t)
	apistest.CheckConditionOngoing(pac, PostReconciler, t)

	// Dependencies installed
	pac.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(pac, DependenciesInstalled, t)

	// Pre reconciler completes execution
	pac.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(pac, PreReconciler, t)

	// Installer set created
	pac.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(pac, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	pac.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(pac, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	pac.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(pac, InstallerSetReady, t)

	pac.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(pac, PostReconciler, t)

	if ready := pac.IsReady(); !ready {
		t.Errorf("pac.IsReady() = %v, want true", ready)
	}
}

func TestOpenShiftPipelinesAsCodeErrorPath(t *testing.T) {
	pac := &OpenShiftPipelinesAsCodeStatus{}
	pac.InitializeConditions()

	apistest.CheckConditionOngoing(pac, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(pac, PreReconciler, t)
	apistest.CheckConditionOngoing(pac, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(pac, InstallerSetReady, t)
	apistest.CheckConditionOngoing(pac, PostReconciler, t)

	// Dependencies installed
	pac.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(pac, DependenciesInstalled, t)

	// Pre reconciler completes execution
	pac.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(pac, PreReconciler, t)

	// Installer set created
	pac.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(pac, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	pac.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(pac, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	pac.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(pac, InstallerSetReady, t)

	pac.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(pac, PostReconciler, t)

	if ready := pac.IsReady(); !ready {
		t.Errorf("pac.IsReady() = %v, want true", ready)
	}

	// In further reconciliation deployment might fail and installer
	// set will change to not ready

	pac.MarkInstallerSetNotReady("webhook not ready")
	apistest.CheckConditionFailed(pac, InstallerSetReady, t)
	if ready := pac.IsReady(); ready {
		t.Errorf("pac.IsReady() = %v, want false", ready)
	}
}
