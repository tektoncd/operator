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

func TestSyncerServiceStatus_SuccessConditions(t *testing.T) {
	ss := &SyncerServiceStatus{}
	ss.InitializeConditions()

	apistest.CheckConditionOngoing(ss, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(ss, PreReconciler, t)
	apistest.CheckConditionOngoing(ss, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(ss, InstallerSetReady, t)
	apistest.CheckConditionOngoing(ss, PostReconciler, t)

	// Dependencies installed
	ss.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(ss, DependenciesInstalled, t)

	// Pre reconciler completes execution
	ss.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(ss, PreReconciler, t)

	// Installer set created
	ss.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(ss, InstallerSetAvailable, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	ss.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(ss, InstallerSetReady, t)

	ss.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(ss, PostReconciler, t)

	if ready := ss.IsReady(); !ready {
		t.Errorf("ss.IsReady() = %v, want true", ready)
	}
}

func TestSyncerServiceStatus_ErrorConditions(t *testing.T) {
	ss := &SyncerServiceStatus{}

	// Not Ready Condition
	ss.MarkNotReady("SyncerService Not Ready")
	apistest.CheckConditionFailed(ss, apis.ConditionReady, t)

	// InstallerSetNotAvailable Condition
	ss.MarkInstallerSetNotAvailable("SyncerService InstallerSetNotAvailable")
	apistest.CheckConditionFailed(ss, InstallerSetAvailable, t)

	// InstallerSetNotReady Condition
	ss.MarkInstallerSetNotReady("SyncerService InstallerSetNotReady")
	apistest.CheckConditionFailed(ss, InstallerSetReady, t)

	// DependencyInstalling Condition
	ss.MarkDependencyInstalling("SyncerService Dependencies are installing")
	apistest.CheckConditionFailed(ss, DependenciesInstalled, t)

	// DependencyMissing Condition
	ss.MarkDependencyMissing("SyncerService Dependencies are Missing")
	apistest.CheckConditionFailed(ss, DependenciesInstalled, t)
}
