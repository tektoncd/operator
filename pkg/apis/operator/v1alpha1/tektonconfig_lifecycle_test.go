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

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apistest "knative.dev/pkg/apis/testing"
)

func TestTektonConfigGroupVersionKind(t *testing.T) {
	r := &TektonConfig{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    KindTektonConfig,
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestTektonConfigHappyPath(t *testing.T) {
	tc := &TektonConfigStatus{}
	tc.InitializeConditions()

	apistest.CheckConditionOngoing(tc, PreInstall, t)
	apistest.CheckConditionOngoing(tc, ComponentsReady, t)
	apistest.CheckConditionOngoing(tc, PostInstall, t)
	apistest.CheckConditionOngoing(tc, PreUpgrade, t)
	apistest.CheckConditionOngoing(tc, PostUpgrade, t)

	// Pre install completes execution
	tc.MarkPreInstallComplete()
	apistest.CheckConditionSucceeded(tc, PreInstall, t)

	// Components and then PostInstall completes and we're good.
	tc.MarkComponentsReady()
	apistest.CheckConditionSucceeded(tc, ComponentsReady, t)

	tc.MarkPostInstallComplete()
	apistest.CheckConditionSucceeded(tc, PostInstall, t)

	status := tc.MarkPreUpgradeComplete()
	assert.Equal(t, true, status)
	// returns false, as upgrade status already up to date
	status = tc.MarkPreUpgradeComplete()
	assert.Equal(t, false, status)
	apistest.CheckConditionSucceeded(tc, PreUpgrade, t)

	status = tc.MarkPostUpgradeComplete()
	assert.Equal(t, true, status)
	// returns false, as upgrade status already up to date
	status = tc.MarkPostUpgradeComplete()
	assert.Equal(t, false, status)
	apistest.CheckConditionSucceeded(tc, PostUpgrade, t)

	if ready := tc.IsReady(); !ready {
		t.Errorf("tc.IsReady() = %v, want true", ready)
	}

}

func TestTektonConfigErrorPath(t *testing.T) {
	tc := &TektonConfigStatus{}
	tc.InitializeConditions()

	apistest.CheckConditionOngoing(tc, PreInstall, t)
	apistest.CheckConditionOngoing(tc, ComponentsReady, t)
	apistest.CheckConditionOngoing(tc, PostInstall, t)
	apistest.CheckConditionOngoing(tc, PreUpgrade, t)
	apistest.CheckConditionOngoing(tc, PostUpgrade, t)

	// Pre install completes execution
	tc.MarkPreInstallComplete()
	apistest.CheckConditionSucceeded(tc, PreInstall, t)

	// ComponentsReady is not ready when components are not in ready state
	tc.MarkComponentNotReady("waiting for component")
	apistest.CheckConditionFailed(tc, ComponentsReady, t)

	// ComponentsReady and then PostInstall become ready and we're good.
	tc.MarkComponentsReady()
	apistest.CheckConditionSucceeded(tc, ComponentsReady, t)

	tc.MarkPostInstallComplete()
	apistest.CheckConditionSucceeded(tc, PostInstall, t)

	status := tc.MarkPreUpgradeComplete()
	assert.Equal(t, true, status)
	apistest.CheckConditionSucceeded(tc, PreUpgrade, t)

	status = tc.MarkPostUpgradeComplete()
	assert.Equal(t, true, status)
	apistest.CheckConditionSucceeded(tc, PostUpgrade, t)

	if ready := tc.IsReady(); !ready {
		t.Errorf("tc.IsReady() = %v, want true", ready)
	}

	// In further reconciliation component might fail

	tc.MarkComponentNotReady("pipeline not ready")
	apistest.CheckConditionFailed(tc, ComponentsReady, t)
	if ready := tc.IsReady(); ready {
		t.Errorf("tc.IsReady() = %v, want false", ready)
	}

}

func TestPreUpgradeVersion(t *testing.T) {
	tc := &TektonConfig{}

	// should return empty
	assert.Equal(t, tc.Status.GetPreUpgradeVersion(), "")

	// update pre upgrade version
	tc.Status.SetPreUpgradeVersion("foo")
	assert.Equal(t, tc.Status.GetPreUpgradeVersion(), "foo")
	assert.Equal(t, tc.Status.Annotations[PreUpgradeVersionKey], "foo")

	// update pre upgrade version
	tc.Status.SetPreUpgradeVersion("bar")
	assert.Equal(t, tc.Status.GetPreUpgradeVersion(), "bar")
	assert.Equal(t, tc.Status.Annotations[PreUpgradeVersionKey], "bar")
}

func TestPostUpgradeVersion(t *testing.T) {
	tc := &TektonConfig{}

	// should return empty
	assert.Equal(t, tc.Status.GetPostUpgradeVersion(), "")

	// update post upgrade version
	tc.Status.SetPostUpgradeVersion("foo")
	assert.Equal(t, tc.Status.GetPostUpgradeVersion(), "foo")
	assert.Equal(t, tc.Status.Annotations[PostUpgradeVersionKey], "foo")

	// update post upgrade version
	tc.Status.SetPostUpgradeVersion("bar")
	assert.Equal(t, tc.Status.GetPostUpgradeVersion(), "bar")
	assert.Equal(t, tc.Status.Annotations[PostUpgradeVersionKey], "bar")
}
