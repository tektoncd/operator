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

func TestTektonTriggerGroupVersionKind(t *testing.T) {
	r := &TektonTrigger{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonTrigger,
	}
	if got := r.GroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonTriggerHappyPath(t *testing.T) {
	tt := &TektonTriggerStatus{}
	tt.InitializeConditions()

	apistest.CheckConditionOngoing(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallSucceeded, t)

	// Install succeeds.
	tt.MarkInstallSucceeded()
	// Dependencies are assumed successful too.
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)

	// Deployments are not available at first.
	tt.MarkDeploymentsNotReady()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionFailed(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}

	// Deployments become ready and we're good.
	tt.MarkDeploymentsAvailable()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionSucceeded(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)
	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestTektonTriggerErrorPath(t *testing.T) {
	tt := &TektonTriggerStatus{}
	tt.InitializeConditions()

	apistest.CheckConditionOngoing(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionOngoing(tt, InstallSucceeded, t)

	// Install fails.
	tt.MarkInstallFailed("test")
	apistest.CheckConditionOngoing(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionFailed(tt, InstallSucceeded, t)

	// Dependencies are installing.
	tt.MarkDependencyInstalling("testing")
	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionFailed(tt, InstallSucceeded, t)

	// Install now succeeds.
	tt.MarkInstallSucceeded()
	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}

	// Deployments become ready
	tt.MarkDeploymentsAvailable()
	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionSucceeded(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)
	if ready := tt.IsReady(); ready {
		t.Errorf("tt.IsReady() = %v, want false", ready)
	}

	// Finally, dependencies become available.
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionSucceeded(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)
	if ready := tt.IsReady(); !ready {
		t.Errorf("tt.IsReady() = %v, want true", ready)
	}
}

func TestTektonTriggerExternalDependency(t *testing.T) {
	tt := &TektonTriggerStatus{}
	tt.InitializeConditions()

	// External marks dependency as failed.
	tt.MarkDependencyMissing("test")

	// Install succeeds.
	tt.MarkInstallSucceeded()
	apistest.CheckConditionFailed(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)

	// Dependencies are now ready.
	tt.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tt, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tt, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tt, InstallSucceeded, t)
}
