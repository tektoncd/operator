/*
Copyright 2019 The Knative Authors

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
	if got := r.GroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonPipelineHappyPath(t *testing.T) {
	tp := &TektonPipelineStatus{}
	tp.InitializeConditions()

	apistest.CheckConditionOngoing(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionOngoing(tp, InstallSucceeded, t)

	tp.MarkVersionMigrationEligible()

	// Install succeeds.
	tp.MarkInstallSucceeded()
	// Dependencies are assumed successful too.
	apistest.CheckConditionSucceeded(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)

	// Deployments are not available at first.
	tp.MarkDeploymentsNotReady()
	apistest.CheckConditionSucceeded(tp, DependenciesInstalled, t)
	apistest.CheckConditionFailed(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)
	if ready := tp.IsReady(); ready {
		t.Errorf("tp.IsReady() = %v, want false", ready)
	}

	// Deployments become ready and we're good.
	tp.MarkDeploymentsAvailable()
	apistest.CheckConditionSucceeded(tp, DependenciesInstalled, t)
	apistest.CheckConditionSucceeded(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)
	if ready := tp.IsReady(); !ready {
		t.Errorf("tp.IsReady() = %v, want true", ready)
	}
}

func TestTektonPipelineErrorPath(t *testing.T) {
	tp := &TektonPipelineStatus{}
	tp.InitializeConditions()

	apistest.CheckConditionOngoing(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionOngoing(tp, InstallSucceeded, t)

	tp.MarkVersionMigrationEligible()

	// Install fails.
	tp.MarkInstallFailed("test")
	apistest.CheckConditionOngoing(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionFailed(tp, InstallSucceeded, t)

	// Dependencies are installing.
	tp.MarkDependencyInstalling("testing")
	apistest.CheckConditionFailed(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionFailed(tp, InstallSucceeded, t)

	// Install now succeeds.
	tp.MarkInstallSucceeded()
	apistest.CheckConditionFailed(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)
	if ready := tp.IsReady(); ready {
		t.Errorf("tp.IsReady() = %v, want false", ready)
	}

	// Deployments become ready
	tp.MarkDeploymentsAvailable()
	apistest.CheckConditionFailed(tp, DependenciesInstalled, t)
	apistest.CheckConditionSucceeded(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)
	if ready := tp.IsReady(); ready {
		t.Errorf("tp.IsReady() = %v, want false", ready)
	}

	// Finally, dependencies become available.
	tp.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tp, DependenciesInstalled, t)
	apistest.CheckConditionSucceeded(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)
	if ready := tp.IsReady(); !ready {
		t.Errorf("tp.IsReady() = %v, want true", ready)
	}
}

func TestTektonPipelineExternalDependency(t *testing.T) {
	tp := &TektonPipelineStatus{}
	tp.InitializeConditions()

	// External marks dependency as failed.
	tp.MarkDependencyMissing("test")

	// Install succeeds.
	tp.MarkInstallSucceeded()
	apistest.CheckConditionFailed(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)

	// Dependencies are now ready.
	tp.MarkDependenciesInstalled()
	apistest.CheckConditionSucceeded(tp, DependenciesInstalled, t)
	apistest.CheckConditionOngoing(tp, DeploymentsAvailable, t)
	apistest.CheckConditionSucceeded(tp, InstallSucceeded, t)
}

func TestTektonPipelineVersionMigrationNotEligible(t *testing.T) {
	tp := &TektonPipelineStatus{}
	tp.InitializeConditions()

	tp.MarkVersionMigrationNotEligible("Version migration not eligible.")
	apistest.CheckConditionFailed(tp, VersionMigrationEligible, t)
}
