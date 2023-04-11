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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

const (
	// DB
	DbDependenciesInstalled apis.ConditionType = "DbDependenciesInstalled"
	DbInstallerSetAvailable apis.ConditionType = "DbInstallSetAvailable"
	// DB-migration
	// TODO: fix the typo on the value: "DatabasebMigrationDone"
	DatabaseMigrationDone apis.ConditionType = "DatabasebMigrationDone"
	// API
	ApiDependenciesInstalled apis.ConditionType = "ApiDependenciesInstalled"
	ApiInstallerSetAvailable apis.ConditionType = "ApiInstallSetAvailable"
	// UI
	UiInstallerSetAvailable apis.ConditionType = "UiInstallSetAvailable"
)

var (
	// TODO: Add this back after refactoring all components
	// and updating TektonComponentStatus to have updated
	// conditions
	// _ TektonComponentStatus = (*TektonHubStatus)(nil)

	hubCondSet = apis.NewLivingConditionSet(
		DbDependenciesInstalled,
		DbInstallerSetAvailable,
		DatabaseMigrationDone,
		PreReconciler,
		ApiDependenciesInstalled,
		ApiInstallerSetAvailable,
		UiInstallerSetAvailable,
		PostReconciler,
	)
)

// GroupVersionKind returns SchemeGroupVersion of a TektonHub
func (th *TektonHub) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonHub)
}

// required by new type of FilterController
// might have to keep this and remove previous or vice-versa
func (th *TektonHub) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(KindTektonHub)
}

// GetCondition returns the current condition of a given condition type
func (ths *TektonHubStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return hubCondSet.Manage(ths).GetCondition(t)
}

// InitializeConditions initializes conditions of an TektonHubStatus
func (ths *TektonHubStatus) InitializeConditions() {
	hubCondSet.Manage(ths).InitializeConditions()
}

func (ths *TektonHubStatus) MarkNotReady(msg string) {
	hubCondSet.Manage(ths).MarkFalse(
		apis.ConditionReady,
		"Error",
		"Ready: %s", msg)
}

// IsReady looks at the conditions returns true if they are all true.
func (ths *TektonHubStatus) IsReady() bool {
	return hubCondSet.Manage(ths).IsHappy()
}

// Lifecycle for the DB component of Tekton Hub
func (ths *TektonHubStatus) MarkDbDependencyInstalling(msg string) {
	ths.MarkNotReady("Dependencies installing for DB")
	hubCondSet.Manage(ths).MarkFalse(
		DbDependenciesInstalled,
		"Error",
		"Dependencies are installing for DB: %s", msg)
}

func (ths *TektonHubStatus) MarkDbDependencyMissing(msg string) {
	ths.MarkNotReady("Missing Dependencies for DB")
	hubCondSet.Manage(ths).MarkFalse(
		DbDependenciesInstalled,
		"Error",
		"Dependencies are missing for DB: %s", msg)
}

func (ths *TektonHubStatus) MarkDbDependenciesInstalled() {
	hubCondSet.Manage(ths).MarkTrue(DbDependenciesInstalled)
}

func (ths *TektonHubStatus) MarkDbInstallerSetNotAvailable(msg string) {
	ths.MarkNotReady("TektonInstallerSet not ready for DB")
	hubCondSet.Manage(ths).MarkFalse(
		DbInstallerSetAvailable,
		"Error",
		"Installer set not ready: %s", msg)
}

func (ths *TektonHubStatus) MarkDbInstallerSetAvailable() {
	hubCondSet.Manage(ths).MarkTrue(DbInstallerSetAvailable)
}

// Lifecycle for the DB migration component of Tekton Hub
func (ths *TektonHubStatus) MarkDatabaseMigrationFailed(msg string) {
	ths.MarkNotReady("Database migration job not ready")
	hubCondSet.Manage(ths).MarkFalse(
		DatabaseMigrationDone,
		"Error",
		"Database migration job not ready: %s", msg)
}

func (ths *TektonHubStatus) MarkDatabaseMigrationDone() {
	hubCondSet.Manage(ths).MarkTrue(DatabaseMigrationDone)
}

// Lifecycle for the API component of Tekton Hub
func (ths *TektonHubStatus) MarkApiDependencyInstalling(msg string) {
	ths.MarkNotReady("Dependencies installing for API")
	hubCondSet.Manage(ths).MarkFalse(
		ApiDependenciesInstalled,
		"Error",
		"Dependencies are installing for API: %s", msg)
}

func (ths *TektonHubStatus) MarkApiDependencyMissing(msg string) {
	ths.MarkNotReady("Missing Dependencies for API")
	hubCondSet.Manage(ths).MarkFalse(
		ApiDependenciesInstalled,
		"Error",
		"Dependencies are missing for API: %s", msg)
}

func (ths *TektonHubStatus) MarkApiDependenciesInstalled() {
	hubCondSet.Manage(ths).MarkTrue(ApiDependenciesInstalled)
}

func (ths *TektonHubStatus) MarkApiInstallerSetNotAvailable(msg string) {
	ths.MarkNotReady("TektonInstallerSet not ready for API")
	hubCondSet.Manage(ths).MarkFalse(
		ApiInstallerSetAvailable,
		"Error",
		"Installer set not ready for API: %s", msg)
}

func (ths *TektonHubStatus) MarkApiInstallerSetAvailable() {
	hubCondSet.Manage(ths).MarkTrue(ApiInstallerSetAvailable)
}

func (ths *TektonHubStatus) MarkUiInstallerSetNotAvailable(msg string) {
	ths.MarkNotReady("TektonInstallerSet not ready for UI")
	hubCondSet.Manage(ths).MarkFalse(
		UiInstallerSetAvailable,
		"Error",
		"Installer set not ready for UI: %s", msg)
}

func (ths *TektonHubStatus) MarkUiInstallerSetAvailable() {
	hubCondSet.Manage(ths).MarkTrue(UiInstallerSetAvailable)
}

// GetManifests gets the url links of the manifests.
func (ths *TektonHubStatus) GetUiRoute() string {
	return ths.UiRouteUrl
}

// SetManifests sets the url links of the manifests.
func (ths *TektonHubStatus) SetUiRoute(routeUrl string) {
	ths.UiRouteUrl = routeUrl
}

func (ths *TektonHubStatus) MarkPreReconcilerFailed(msg string) {
	ths.MarkNotReady("PreReconciliation failed")
	hubCondSet.Manage(ths).MarkFalse(
		PreReconciler,
		"Error",
		"PreReconciliation failed with message: %s", msg)
}

func (ths *TektonHubStatus) MarkPreReconcilerComplete() {
	hubCondSet.Manage(ths).MarkTrue(PreReconciler)
}

func (ths *TektonHubStatus) MarkPostReconcilerFailed(msg string) {
	ths.MarkNotReady("PostReconciliation failed")
	hubCondSet.Manage(ths).MarkFalse(
		PostReconciler,
		"Error",
		"PostReconciliation failed with message: %s", msg)
}

func (ths *TektonHubStatus) MarkPostReconcilerComplete() {
	hubCondSet.Manage(ths).MarkTrue(PostReconciler)
}

// Get the API route URL
func (ths *TektonHubStatus) GetApiRoute() string {
	return ths.ApiRouteUrl
}

// Set the API route URL
func (ths *TektonHubStatus) SetApiRoute(routeUrl string) {
	ths.ApiRouteUrl = routeUrl
}

// Get the Auth route URL
func (ths *TektonHubStatus) GetAuthRoute() string {
	return ths.AuthRouteUrl
}

// Set the Auth route URL
func (ths *TektonHubStatus) SetAuthRoute(routeUrl string) {
	ths.AuthRouteUrl = routeUrl
}

// GetVersion gets the currently installed version of the component.
func (ths *TektonHubStatus) GetVersion() string {
	return ths.Version
}

// SetVersion sets the currently installed version of the component.
func (ths *TektonHubStatus) SetVersion(version string) {
	ths.Version = version
}

// GetManifests gets the url links of the manifests.
func (ths *TektonHubStatus) GetManifests() []string {
	return ths.Manifests
}

// SetManifests sets the url links of the manifests.
func (ths *TektonHubStatus) SetManifests(manifests []string) {
	ths.Manifests = manifests
}
