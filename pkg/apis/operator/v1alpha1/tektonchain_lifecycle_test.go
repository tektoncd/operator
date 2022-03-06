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

func TestTektonChainGroupVersionKind(t *testing.T) {
	r := &TektonChain{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonChain,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonChainHappyPath(t *testing.T) {
	tt := &TektonChainStatus{}
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

func TestTektonChainErrorPath(t *testing.T) {
	tr := &TektonChainStatus{}
	tr.InitializeConditions()

	apistest.CheckConditionOngoing(tr, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tr, PreReconciler, t)
	apistest.CheckConditionOngoing(tr, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tr, InstallerSetReady, t)
	apistest.CheckConditionOngoing(tr, PostReconciler, t)

	// Dependencies installed
	tr.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tr, DependenciesInstalled, t)

	// Pre reconciler completes execution
	tr.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(tr, PreReconciler, t)

	// Installer set created
	tr.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(tr, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	tr.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(tr, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	tr.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tr, InstallerSetReady, t)

	tr.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(tr, PostReconciler, t)

	if ready := tr.IsReady(); !ready {
		t.Errorf("tr.IsReady() = %v, want true", ready)
	}

	// In further reconciliation deployment might fail and installer
	// set will change to not ready

	tr.MarkInstallerSetNotReady("webhook not ready")
	apistest.CheckConditionFailed(tr, InstallerSetReady, t)
	if ready := tr.IsReady(); ready {
		t.Errorf("tr.IsReady() = %v, want false", ready)
	}

}

func TestTektonChainExternalDependency(t *testing.T) {
	tt := &TektonChainStatus{}
	tt.InitializeConditions()

	// External marks dependency as failed.
	tt.MarkDependencyMissing("test")

	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, PreReconciler, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallerSetReady, t)
	apistest.CheckConditionOngoing(tt, PostReconciler, t)

	// Dependencies are now ready.
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, PreReconciler, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallerSetReady, t)
	apistest.CheckConditionOngoing(tt, PostReconciler, t)
}
