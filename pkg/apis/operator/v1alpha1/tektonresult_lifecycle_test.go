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

func TestTektonResultGroupVersionKind(t *testing.T) {
	r := &TektonResult{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonResult,
	}
	if got := r.GroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonResultHappyPath(t *testing.T) {
	tt := &TektonResultStatus{}
	tt.InitializeConditions()

	apistest.CheckConditionOngoing(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallerSetReady, t)

	// Dependencies installed
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)

	tt.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(tt, InstallerSetAvailable, t)

	tt.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tt, InstallerSetReady, t)

	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestTektonResultErrorPath(t *testing.T) {
	tt := &TektonResultStatus{}
	tt.InitializeConditions()

	apistest.CheckConditionOngoing(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallerSetReady, t)

	// Dependencies installed
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)

	tt.MarkInstallerSetAvailable()
	apistest.CheckConditionSucceeded(tt, InstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	tt.MarkInstallerSetNotReady("waiting for deployments")
	apistest.CheckConditionFailed(tt, InstallerSetReady, t)

	// InstallerSet and then PostReconciler become ready and we're good.
	tt.MarkInstallerSetReady()
	apistest.CheckConditionSucceeded(tt, InstallerSetReady, t)

	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestTektonResultExternalDependency(t *testing.T) {
	tt := &TektonResultStatus{}
	tt.InitializeConditions()

	// External marks dependency as failed.
	tt.MarkDependencyMissing("test")
	tt.MarkInstallerSetReady()

	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallerSetReady, t)

	// Dependencies are now ready.
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, InstallerSetAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallerSetReady, t)
}
