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

	"knative.dev/pkg/apis"
	apistest "knative.dev/pkg/apis/testing"
)

func TestTektonSchedulerStatus_SuccessConditions(t *testing.T) {
	tt := &TektonSchedulerStatus{}
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

	// InstallerSet and then PostReconciler become ready and we're good.
	tt.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tt, InstallerSetReady, t)

	tt.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(tt, PostReconciler, t)

	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestTektonSchedulerStatus_ErrorConditions(t *testing.T) {
	// Given
	tps := &TektonSchedulerStatus{}

	tps.MarkPreReconcilerFailed("Reconciliation Failed for Scheduler")
	apistest.CheckConditionFailed(tps, PreReconciler, t)

	// Not Ready Condition
	tps.MarkNotReady("Scheduler Not Ready")
	apistest.CheckConditionFailed(tps, apis.ConditionReady, t)

	// PostReconciler Failed Condition
	tps.MarkPostReconcilerFailed("Scheduler PostReconciler Failed")
	apistest.CheckConditionFailed(tps, PostReconciler, t)

	// InstallerSetNotAvailable Condition
	tps.MarkInstallerSetNotAvailable("Scheduler InstallerSetNotAvailable ")
	apistest.CheckConditionFailed(tps, InstallerSetAvailable, t)

	// InstallerSetNotAvailable Condition
	tps.MarkInstallerSetNotReady("Scheduler InstallerSetNotReady ")
	apistest.CheckConditionFailed(tps, InstallerSetReady, t)

	// DependencyInstalling Condition
	tps.MarkDependencyInstalling("Scheduler Dependencies are installing ")
	apistest.CheckConditionFailed(tps, DependenciesInstalled, t)

	// DependencyMissing Condition
	tps.MarkDependencyMissing("Scheduler Dependencies are Missing ")
	apistest.CheckConditionFailed(tps, DependenciesInstalled, t)
}
