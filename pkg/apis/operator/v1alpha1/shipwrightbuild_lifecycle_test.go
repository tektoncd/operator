/*
Copyright 2026 The Tekton Authors

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

func TestShipwrightBuildGroupVersionKind(t *testing.T) {
	r := &ShipwrightBuild{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindShipwrightBuild,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got := r.GroupVersionKind(); got != want {
		t.Errorf("GroupVersionKind() got: %v, want: %v", got, want)
	}
}

func TestShipwrightBuildInstallerSetAndVersionAccessors(t *testing.T) {
	status := &ShipwrightBuildStatus{}

	status.SetTektonInstallerSet("shipwright-build-main-abcde")
	if got := status.GetTektonInstallerSet(); got != "shipwright-build-main-abcde" {
		t.Errorf("GetTektonInstallerSet() = %q, want %q", got, "shipwright-build-main-abcde")
	}

	status.SetVersion("v0.21.0")
	if got := status.GetVersion(); got != "v0.21.0" {
		t.Errorf("GetVersion() = %q, want %q", got, "v0.21.0")
	}
}

func TestShipwrightBuildHappyPath(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	apistest.CheckConditionOngoing(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, PreReconciler, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallerSetReady, t)
	apistest.CheckConditionOngoing(tt, PostReconciler, t)

	// Dependencies installed
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)

	// Pre reconciler completes execution
	tt.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(tt, PreReconciler, t)

	// Installer set created
	tt.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(tt, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	tt.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(tt, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	tt.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tt, InstallerSetReady, t)

	tt.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(tt, PostReconciler, t)

	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestShipwrightBuildErrorPath(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	// Reach the ready state first.
	tt.MarkDependenciesInstalled()
	tt.MarkPreReconcilerComplete()
	tt.MarkInstallerSetAvailable()
	tt.MarkInstallerSetReady()
	tt.MarkPostReconcilerComplete()
	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}

	// A later reconciliation finds the installer set no longer ready.
	tt.MarkInstallerSetNotReady("webhook not ready")
	apistest.CheckConditionFailed(tt, InstallerSetReady, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}
}

func TestShipwrightBuildExternalDependency(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	// External marks dependency as failed.
	tt.MarkDependencyMissing("test")
	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, PreReconciler, t)

	// Dependencies are now ready.
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
}

func TestShipwrightBuildDependencyInstalling(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	// A dependency still installing marks DependenciesInstalled as failed.
	tt.MarkDependencyInstalling("installing")
	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, PreReconciler, t)

	// Once installed, the condition recovers.
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
}

func TestShipwrightBuildPreReconcilerFailed(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	tt.MarkPreReconcilerFailed("pre-reconcile boom")
	apistest.CheckConditionFailed(tt, PreReconciler, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}
}

func TestShipwrightBuildPostReconcilerFailed(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	tt.MarkPostReconcilerFailed("post-reconcile boom")
	apistest.CheckConditionFailed(tt, PostReconciler, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}
}

func TestShipwrightBuildInstallerSetNotAvailable(t *testing.T) {
	tt := &ShipwrightBuildStatus{}
	tt.InitializeConditions()

	tt.MarkInstallerSetNotAvailable("installer set deleted")
	apistest.CheckConditionFailed(tt, InstallerSetAvailable, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}
}
