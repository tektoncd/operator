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
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	apistest "knative.dev/pkg/apis/testing"
)

func TestTektonPipelineGroupVersionKind(t *testing.T) {
	r := &TektonPipeline{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonPipeline,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonPipelineHappyPath(t *testing.T) {
	tp := &TektonPipelineStatus{}
	tp.InitializeConditions()

	apistest.CheckConditionOngoing(tp, PreReconciler, t)
	apistest.CheckConditionOngoing(tp, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tp, InstallerSetReady, t)
	apistest.CheckConditionOngoing(tp, PostReconciler, t)

	// Pre reconciler completes execution
	tp.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(tp, PreReconciler, t)

	// Installer set created
	tp.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(tp, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	tp.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(tp, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	tp.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tp, InstallerSetReady, t)

	tp.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(tp, PostReconciler, t)

	if ready := tp.IsReady(); !ready {
		t.Errorf("tp.IsReady() = %v, want true", ready)
	}
}

func TestTektonPipelineErrorPath(t *testing.T) {
	tp := &TektonPipelineStatus{}
	tp.InitializeConditions()

	apistest.CheckConditionOngoing(tp, PreReconciler, t)
	apistest.CheckConditionOngoing(tp, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tp, InstallerSetReady, t)
	apistest.CheckConditionOngoing(tp, PostReconciler, t)

	// Pre reconciler completes execution
	tp.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(tp, PreReconciler, t)

	// Installer set created
	tp.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(tp, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	tp.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(tp, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	tp.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tp, InstallerSetReady, t)

	tp.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(tp, PostReconciler, t)

	if ready := tp.IsReady(); !ready {
		t.Errorf("tp.IsReady() = %v, want true", ready)
	}

	// In further reconciliation deployment might fail and installer
	// set will change to not ready

	tp.MarkInstallerSetNotReady("webhook not ready")
	apistest.CheckConditionFailed(tp, InstallerSetReady, t)
	if ready := tp.IsReady(); ready {
		t.Errorf("tp.IsReady() = %v, want false", ready)
	}
}
