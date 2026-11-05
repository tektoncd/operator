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

package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
)

func TestRunPreUpgrade_AutomaticallyAdoptsLegacyBuilds(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")
	ctx := context.Background()
	parent := legacyBuildParent("Enabled")
	assert.NilError(t, unstructured.SetNestedField(parent.Object, "Enabled", "spec", "sharedResource", "state"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/apis/operator.openshift.io/v1alpha1/openshiftbuilds/cluster" {
			t.Errorf("unexpected legacy API request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(parent); err != nil {
			t.Errorf("encode legacy parent: %v", err)
		}
	}))
	defer server.Close()
	tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
	tc.Spec.TargetNamespace = "openshift-pipelines"
	tc.Status.SetPreUpgradeVersion("1.24.3")
	operatorClient := operatorfake.NewSimpleClientset(tc)
	ug := New("1.24.4", kubefake.NewSimpleClientset(), operatorClient, &rest.Config{Host: server.URL})
	// Exercise the real registered upgrade chain, not a manual adoption call.
	for attempt := 0; attempt < 10; attempt++ {
		err := ug.RunPreUpgrade(ctx)
		if err == nil {
			break
		}
		assert.Assert(t, errors.Is(err, v1alpha1.RECONCILE_AGAIN_ERR) || errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER), "unexpected upgrade error: %v", err)
	}
	updated, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, tc.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, updated.Status.GetPreUpgradeVersion(), "1.24.4")
	assert.Assert(t, updated.Spec.Platforms.OpenShift != nil)
	assert.Assert(t, updated.Spec.Platforms.OpenShift.ShipwrightBuild != nil)
	assert.Assert(t, updated.Spec.Platforms.OpenShift.SharedResource != nil)
	assert.Assert(t, updated.Spec.Platforms.OpenShift.ShipwrightBuild.Enabled)
	assert.Assert(t, updated.Spec.Platforms.OpenShift.SharedResource.Enabled)
}

func TestAdoptLegacyBuilds_ComponentStates(t *testing.T) {
	for _, shipwright := range []string{"Enabled", "Disabled", ""} {
		for _, csi := range []string{"Enabled", "Disabled", ""} {
			t.Run(shipwright+"/"+csi, func(t *testing.T) {
				ctx := context.Background()
				parent := legacyBuildParent(shipwright)
				assert.NilError(t, unstructured.SetNestedField(parent.Object, csi, "spec", "sharedResource", "state"))
				tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
				client := operatorfake.NewSimpleClientset(tc)
				assert.NilError(t, adoptLegacyBuilds(ctx, client, dynfake.NewSimpleDynamicClient(runtime.NewScheme(), parent)))
				updated, err := client.OperatorV1alpha1().TektonConfigs().Get(ctx, tc.Name, metav1.GetOptions{})
				assert.NilError(t, err)
				platform := updated.Spec.Platforms.OpenShift
				if shipwright == "Disabled" && csi == "Disabled" {
					assert.Assert(t, platform == nil)
					return
				}
				assert.Assert(t, platform != nil)
				assert.Equal(t, platform.ShipwrightBuild != nil && platform.ShipwrightBuild.Enabled, shipwright != "Disabled")
				assert.Equal(t, platform.SharedResource != nil && platform.SharedResource.Enabled, csi != "Disabled")
			})
		}
	}
}

func TestAdoptLegacyBuilds_ExplicitSettingsAreIndependent(t *testing.T) {
	for _, explicitShipwright := range []bool{false, true} {
		t.Run(map[bool]string{true: "shipwright", false: "csi"}[explicitShipwright], func(t *testing.T) {
			ctx := context.Background()
			parent := legacyBuildParent("Enabled")
			assert.NilError(t, unstructured.SetNestedField(parent.Object, "Enabled", "spec", "sharedResource", "state"))
			tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
			tc.Spec.Platforms.OpenShift = &v1alpha1.OpenShift{}
			if explicitShipwright {
				tc.Spec.Platforms.OpenShift.ShipwrightBuild = &v1alpha1.ShipwrightBuildConfig{Enabled: false}
			} else {
				tc.Spec.Platforms.OpenShift.SharedResource = &v1alpha1.SharedResourceConfig{Enabled: false}
			}
			client := operatorfake.NewSimpleClientset(tc)
			assert.NilError(t, adoptLegacyBuilds(ctx, client, dynfake.NewSimpleDynamicClient(runtime.NewScheme(), parent)))
			updated, err := client.OperatorV1alpha1().TektonConfigs().Get(ctx, tc.Name, metav1.GetOptions{})
			assert.NilError(t, err)
			assert.Equal(t, updated.Spec.Platforms.OpenShift.ShipwrightBuild.Enabled, !explicitShipwright)
			assert.Equal(t, updated.Spec.Platforms.OpenShift.SharedResource.Enabled, explicitShipwright)
		})
	}
}

func TestPreUpgradeLegacyBuilds_KubernetesIsNoop(t *testing.T) {
	t.Setenv("PLATFORM", "kubernetes")
	assert.NilError(t, preUpgradeLegacyBuilds(context.Background(), nil, nil, nil, nil))
}

func TestAdoptLegacyShipwright_AbsentTektonConfig(t *testing.T) {
	operatorClient := operatorfake.NewSimpleClientset()
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildParent("Enabled"))
	assert.NilError(t, adoptLegacyBuilds(context.Background(), operatorClient, dynamicClient))
	assert.Equal(t, len(dynamicClient.Actions()), 0)
}

func TestAdoptLegacyShipwright_InvalidLegacyParent(t *testing.T) {
	for _, scenario := range []string{"terminating", "unknown-state", "invalid-state-type"} {
		t.Run(scenario, func(t *testing.T) {
			parent := legacyBuildParent("Enabled")
			switch scenario {
			case "terminating":
				now := metav1.Now()
				parent.SetDeletionTimestamp(&now)
			case "unknown-state":
				assert.NilError(t, unstructured.SetNestedField(parent.Object, "Unknown", "spec", "shipwright", "build", "state"))
			case "invalid-state-type":
				assert.NilError(t, unstructured.SetNestedField(parent.Object, true, "spec", "shipwright", "build", "state"))
			}
			tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
			operatorClient := operatorfake.NewSimpleClientset(tc)
			dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), parent)
			assert.Assert(t, adoptLegacyBuilds(context.Background(), operatorClient, dynamicClient) != nil)
			for _, action := range operatorClient.Actions() {
				assert.Equal(t, action.GetVerb(), "get")
			}
		})
	}
}

func TestAdoptLegacyShipwright_PreservesPlatformSettings(t *testing.T) {
	tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
	tc.Spec.Platforms.OpenShift = &v1alpha1.OpenShift{SharedResource: &v1alpha1.SharedResourceConfig{Enabled: true}}
	operatorClient := operatorfake.NewSimpleClientset(tc)
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildParent("Enabled"))
	assert.NilError(t, adoptLegacyBuilds(context.Background(), operatorClient, dynamicClient))
	updated, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(context.Background(), tc.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, updated.Spec.Platforms.OpenShift.SharedResource.Enabled)
	assert.Assert(t, updated.Spec.Platforms.OpenShift.ShipwrightBuild.Enabled)
}

func legacyBuildParent(state string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.openshift.io/v1alpha1", "kind": "OpenShiftBuild",
		"metadata": map[string]interface{}{"name": "cluster"},
		"spec": map[string]interface{}{
			"shipwright":     map[string]interface{}{"build": map[string]interface{}{"state": state}},
			"sharedResource": map[string]interface{}{"state": "Disabled"},
		},
	}}
}

func TestAdoptLegacyShipwright(t *testing.T) {
	for _, state := range []string{"Enabled", "", "Disabled"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
			tc.Spec.TargetNamespace = "openshift-pipelines"
			operatorClient := operatorfake.NewSimpleClientset(tc)
			dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildParent(state))
			assert.NilError(t, adoptLegacyBuilds(ctx, operatorClient, dynamicClient))
			updated, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, tc.Name, metav1.GetOptions{})
			assert.NilError(t, err)
			if state == "Disabled" {
				assert.Assert(t, updated.Spec.Platforms.OpenShift == nil)
			} else {
				assert.Assert(t, updated.Spec.Platforms.OpenShift.ShipwrightBuild.Enabled)
				assert.Assert(t, updated.Spec.Platforms.OpenShift.SharedResource == nil)
			}
			assert.Equal(t, updated.Spec.TargetNamespace, tc.Spec.TargetNamespace)
			operatorClient.ClearActions()
			assert.NilError(t, adoptLegacyBuilds(ctx, operatorClient, dynamicClient))
			for _, action := range operatorClient.Actions() {
				assert.Equal(t, action.GetVerb(), "get")
			}
			for _, action := range dynamicClient.Actions() {
				assert.Equal(t, action.GetVerb(), "get")
			}
		})
	}
}

func TestAdoptLegacyShipwright_RespectsExplicitSettings(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
		tc.Spec.Platforms.OpenShift = &v1alpha1.OpenShift{
			ShipwrightBuild: &v1alpha1.ShipwrightBuildConfig{Enabled: enabled},
			SharedResource:  &v1alpha1.SharedResourceConfig{Enabled: !enabled},
		}
		operatorClient := operatorfake.NewSimpleClientset(tc)
		dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildParent("Enabled"))
		assert.NilError(t, adoptLegacyBuilds(context.Background(), operatorClient, dynamicClient))
		assert.Equal(t, len(dynamicClient.Actions()), 0)
		for _, action := range operatorClient.Actions() {
			assert.Equal(t, action.GetVerb(), "get")
		}
	}
}

func TestAdoptLegacyShipwright_AbsentLegacyIsFreshInstall(t *testing.T) {
	tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
	operatorClient := operatorfake.NewSimpleClientset(tc)
	assert.NilError(t, adoptLegacyBuilds(context.Background(), operatorClient, dynfake.NewSimpleDynamicClient(runtime.NewScheme())))
	for _, action := range operatorClient.Actions() {
		assert.Equal(t, action.GetVerb(), "get")
	}
}

func TestAdoptLegacyShipwright_PropagatesFailures(t *testing.T) {
	for _, stage := range []string{"detection", "update"} {
		t.Run(stage, func(t *testing.T) {
			tc := &v1alpha1.TektonConfig{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName}}
			operatorClient := operatorfake.NewSimpleClientset(tc)
			dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), legacyBuildParent("Enabled"))
			wantErr := apierrors.NewConflict(schema.GroupResource{Resource: "tektonconfigs"}, tc.Name, errors.New("retry"))
			reactor := func(clienttesting.Action) (bool, runtime.Object, error) { return true, nil, wantErr }
			if stage == "detection" {
				dynamicClient.PrependReactor("get", "openshiftbuilds", reactor)
			} else {
				operatorClient.PrependReactor("update", "tektonconfigs", reactor)
			}
			assert.Assert(t, errors.Is(adoptLegacyBuilds(context.Background(), operatorClient, dynamicClient), wantErr))
		})
	}
}
