package v1alpha1

import (
	"testing"

	"knative.dev/pkg/apis"
	apistest "knative.dev/pkg/apis/testing"
)

func TestTektonPrunerStatus_SuccessConditions(t *testing.T) {
	tt := &TektonPrunerStatus{}
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

func TestTektonPrunerStatus_ErrorConditions(t *testing.T) {
	// Given
	tps := &TektonPrunerStatus{}

	tps.MarkPreReconcilerFailed("Reconciliation Failed for Pruner")
	apistest.CheckConditionFailed(tps, PreReconciler, t)

	// Not Ready Condition
	tps.MarkNotReady("Pruner Not Ready")
	apistest.CheckConditionFailed(tps, apis.ConditionReady, t)

	// PostReconciler Failed Condition
	tps.MarkPostReconcilerFailed("Pruner PostReconciler Failed")
	apistest.CheckConditionFailed(tps, PostReconciler, t)

	// InstallerSetNotAvailable Condition
	tps.MarkInstallerSetNotAvailable("Pruner InstallerSetNotAvailable ")
	apistest.CheckConditionFailed(tps, InstallerSetAvailable, t)

	// InstallerSetNotAvailable Condition
	tps.MarkInstallerSetNotReady("Pruner InstallerSetNotReady ")
	apistest.CheckConditionFailed(tps, InstallerSetReady, t)

	// DependencyInstalling Condition
	tps.MarkDependencyInstalling("Pruner Dependencies are installing ")
	apistest.CheckConditionFailed(tps, DependenciesInstalled, t)

	// DependencyMissing Condition
	tps.MarkDependencyMissing("Pruner Dependencies are Missing ")
	apistest.CheckConditionFailed(tps, DependenciesInstalled, t)
}
