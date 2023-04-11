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

func TestTektonHubGroupVersionKind(t *testing.T) {
	r := &TektonHub{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonHub,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonHubHappyPath(t *testing.T) {
	th := &TektonHubStatus{}
	th.InitializeConditions()

	apistest.CheckConditionOngoing(th, DbDependenciesInstalled, t)
	apistest.CheckConditionOngoing(th, DbInstallerSetAvailable, t)
	apistest.CheckConditionOngoing(th, DatabaseMigrationDone, t)
	apistest.CheckConditionOngoing(th, ApiDependenciesInstalled, t)
	apistest.CheckConditionOngoing(th, PreReconciler, t)
	apistest.CheckConditionOngoing(th, ApiInstallerSetAvailable, t)
	apistest.CheckConditionOngoing(th, PostReconciler, t)

	// DB
	// DB dependencies are created
	th.MarkDbDependenciesInstalled()
	apistest.CheckConditionSucceeded(th, DbDependenciesInstalled, t)

	// InstallerSet is not ready when deployment pods are not up
	th.MarkDbInstallerSetNotAvailable("waiting for DB deployments")
	apistest.CheckConditionFailed(th, DbInstallerSetAvailable, t)

	// Installer set created for DB
	th.MarkDbInstallerSetAvailable()
	apistest.CheckConditionSucceeded(th, DbInstallerSetAvailable, t)

	// Db-migration
	// InstallerSet is not ready when Job pods are not up
	th.MarkDatabaseMigrationFailed("waiting for Job to complete")
	apistest.CheckConditionFailed(th, DatabaseMigrationDone, t)

	// Installer set created for DB migration
	th.MarkDatabaseMigrationDone()
	apistest.CheckConditionSucceeded(th, DatabaseMigrationDone, t)

	//API

	th.MarkApiDependenciesInstalled()
	apistest.CheckConditionSucceeded(th, ApiDependenciesInstalled, t)

	// InstallerSet is not ready when deployment pods are not up
	th.MarkUiInstallerSetNotAvailable("waiting for UI deployments")
	apistest.CheckConditionFailed(th, UiInstallerSetAvailable, t)

	// Installer set created for UI
	th.MarkUiInstallerSetAvailable()
	apistest.CheckConditionSucceeded(th, UiInstallerSetAvailable, t)

	th.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(th, PreReconciler, t)

	// InstallerSet is not ready when deployment pods are not up
	th.MarkApiInstallerSetNotAvailable("waiting for API deployments")
	apistest.CheckConditionFailed(th, ApiInstallerSetAvailable, t)

	// Installer set created for API
	th.MarkApiInstallerSetAvailable()
	apistest.CheckConditionSucceeded(th, ApiInstallerSetAvailable, t)

	th.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(th, PostReconciler, t)

	if ready := th.IsReady(); !ready {
		t.Errorf("tp.IsReady() = %v, want true", ready)
	}
}

func TestTektonHubErrorPath(t *testing.T) {
	th := &TektonHubStatus{}
	th.InitializeConditions()

	apistest.CheckConditionOngoing(th, DbDependenciesInstalled, t)
	apistest.CheckConditionOngoing(th, DbInstallerSetAvailable, t)
	apistest.CheckConditionOngoing(th, DatabaseMigrationDone, t)
	apistest.CheckConditionOngoing(th, ApiDependenciesInstalled, t)
	apistest.CheckConditionOngoing(th, PreReconciler, t)
	apistest.CheckConditionOngoing(th, ApiInstallerSetAvailable, t)
	apistest.CheckConditionOngoing(th, PostReconciler, t)

	// DB dependencies are created
	th.MarkDbDependenciesInstalled()
	apistest.CheckConditionSucceeded(th, DbDependenciesInstalled, t)

	// InstallerSet is not ready when deployment pods are not up
	th.MarkDbInstallerSetNotAvailable("waiting for DB deployments")
	apistest.CheckConditionFailed(th, DbInstallerSetAvailable, t)

	// Installer set created for DB
	th.MarkDbInstallerSetAvailable()
	apistest.CheckConditionSucceeded(th, DbInstallerSetAvailable, t)

	// InstallerSet is not ready when Job pods are not up
	th.MarkDatabaseMigrationFailed("waiting for Job to complete")
	apistest.CheckConditionFailed(th, DatabaseMigrationDone, t)

	// Installer set created for DB migration
	th.MarkDatabaseMigrationDone()
	apistest.CheckConditionSucceeded(th, DatabaseMigrationDone, t)

	th.MarkApiDependenciesInstalled()
	apistest.CheckConditionSucceeded(th, ApiDependenciesInstalled, t)

	th.MarkPreReconcilerComplete()
	apistest.CheckConditionSucceeded(th, PreReconciler, t)

	// InstallerSet is not ready when deployment pods are not up
	th.MarkApiInstallerSetNotAvailable("waiting for API deployments")
	apistest.CheckConditionFailed(th, ApiInstallerSetAvailable, t)

	// Installer set created for API
	th.MarkApiInstallerSetAvailable()
	apistest.CheckConditionSucceeded(th, ApiInstallerSetAvailable, t)

	// InstallerSet is not ready when deployment pods are not up
	th.MarkUiInstallerSetNotAvailable("waiting for UI deployments")
	apistest.CheckConditionFailed(th, UiInstallerSetAvailable, t)

	// Installer set created for UI
	th.MarkUiInstallerSetAvailable()
	apistest.CheckConditionSucceeded(th, UiInstallerSetAvailable, t)

	th.MarkPostReconcilerComplete()
	apistest.CheckConditionSucceeded(th, PostReconciler, t)

	// In further reconciliation deployment might fail and installer
	// set will change to not ready

	th.MarkApiInstallerSetNotAvailable("waiting for API deployments")
	apistest.CheckConditionFailed(th, ApiInstallerSetAvailable, t)
	if ready := th.IsReady(); ready {
		t.Errorf("tp.IsReady() = %v, want false", ready)
	}
}
