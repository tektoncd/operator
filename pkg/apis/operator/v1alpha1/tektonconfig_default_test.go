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
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	dynamicclient "github.com/tektoncd/operator/pkg/client/injection/dynamic"
	"github.com/tektoncd/tekton-kueue/pkg/config"

	"gotest.tools/v3/assert"
	"knative.dev/pkg/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func openShiftBuildContext(t *testing.T, shipwrightState, sharedResourceState string, found bool) context.Context {
	t.Helper()
	var objects []runtime.Object
	if found {
		build := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "OpenShiftBuild",
			"metadata":   map[string]interface{}{"name": "cluster"},
		}}
		if shipwrightState != "" {
			assert.NilError(t, unstructured.SetNestedField(build.Object, shipwrightState, "spec", "shipwright", "build", "state"))
		}
		if sharedResourceState != "" {
			assert.NilError(t, unstructured.SetNestedField(build.Object, sharedResourceState, "spec", "sharedResource", "state"))
		}
		objects = append(objects, build)
	}
	return dynamicclient.WithClient(context.Background(), dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), objects...))
}

func Test_SetDefaults_OpenShiftBuildComponents(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")
	for _, tc := range []struct {
		name                string
		shipwrightState     string
		sharedResourceState string
		found               bool
		enabled             bool
	}{
		{name: "both standalone components enabled", shipwrightState: "Enabled", sharedResourceState: "Enabled", found: true},
		{name: "standalone defaults enabled", found: true},
		{name: "only standalone CSI enabled", shipwrightState: "Disabled", sharedResourceState: "Enabled", found: true},
		{name: "only standalone Shipwright enabled", shipwrightState: "Enabled", sharedResourceState: "Disabled", found: true},
		{name: "both standalone components disabled", shipwrightState: "Disabled", sharedResourceState: "Disabled", found: true},
		{name: "standalone absent", enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &TektonConfig{}
			ctx := openShiftBuildContext(t, tc.shipwrightState, tc.sharedResourceState, tc.found)
			config.SetDefaults(ctx)
			assert.Equal(t, config.Spec.Platforms.OpenShift.ShipwrightBuild.Enabled, tc.enabled)
			assert.Equal(t, config.Spec.Platforms.OpenShift.SharedResource.Enabled, tc.enabled)
			client := dynamicclient.Get(ctx).(*dynamicfake.FakeDynamicClient)
			assert.Equal(t, len(client.Actions()), 1, "both defaults must share one legacy CR lookup")
		})
	}
}

func Test_SetDefaults_OpenShiftBuildComponentsPreserveExplicitValues(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")
	for _, tc := range []struct {
		name       string
		found      bool
		configured bool
	}{
		{name: "explicit enable with standalone Builds", found: true, configured: true},
		{name: "explicit disable without standalone Builds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			platform := &OpenShift{
				ShipwrightBuild: &ShipwrightBuildConfig{Enabled: tc.configured},
				SharedResource:  &SharedResourceConfig{Enabled: tc.configured},
			}
			config := &TektonConfig{Spec: TektonConfigSpec{Platforms: Platforms{OpenShift: platform}}}
			ctx := openShiftBuildContext(t, "Enabled", "Enabled", tc.found)
			config.SetDefaults(ctx)
			assert.Equal(t, platform.ShipwrightBuild.Enabled, tc.configured)
			assert.Equal(t, platform.SharedResource.Enabled, tc.configured)
			client := dynamicclient.Get(ctx).(*dynamicfake.FakeDynamicClient)
			assert.Equal(t, len(client.Actions()), 0, "explicit component blocks must skip detection")
		})
	}
}

func Test_SetDefaults_OpenShiftBuildComponentsPreserveOneExplicitValue(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")
	platform := &OpenShift{ShipwrightBuild: &ShipwrightBuildConfig{Enabled: true}}
	config := &TektonConfig{Spec: TektonConfigSpec{Platforms: Platforms{OpenShift: platform}}}
	ctx := openShiftBuildContext(t, "Disabled", "Enabled", true)
	config.SetDefaults(ctx)
	assert.Equal(t, platform.ShipwrightBuild.Enabled, true)
	assert.Equal(t, platform.SharedResource.Enabled, false)
}

func Test_SetDefaults_KubernetesSkipsStandaloneBuildsDetection(t *testing.T) {
	t.Setenv("PLATFORM", "kubernetes")
	ctx := openShiftBuildContext(t, "Enabled", "Enabled", true)
	config := &TektonConfig{}
	config.SetDefaults(ctx)
	assert.Assert(t, config.Spec.Platforms.OpenShift == nil)
	assert.Equal(t, config.Spec.Platforms.Kubernetes.ShipwrightBuild.Enabled, false)
	client := dynamicclient.Get(ctx).(*dynamicfake.FakeDynamicClient)
	assert.Equal(t, len(client.Actions()), 0)
}

func Test_SetDefaults_OpenShift_MigratesKubernetesPipelinesAsCode(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")
	kpac := &PipelinesAsCode{
		Enable: ptr.Bool(true),
		PACSettings: PACSettings{
			Settings: map[string]string{"application-name": "test"},
		},
	}
	tc := &TektonConfig{
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{TargetNamespace: "ns"},
			Platforms: Platforms{
				Kubernetes: &Kubernetes{PipelinesAsCode: kpac},
			},
		},
	}
	tc.SetDefaults(context.TODO())
	if tc.Spec.Platforms.Kubernetes != nil {
		t.Fatalf("expected kubernetes platform nil after migration, got %+v", tc.Spec.Platforms.Kubernetes)
	}
	if tc.Spec.Platforms.OpenShift == nil || tc.Spec.Platforms.OpenShift.PipelinesAsCode == nil || tc.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings["application-name"] != "test" {
		t.Fatalf("expected PAC migrated to openshift, got %+v", tc.Spec.Platforms.OpenShift)
	}
}

func Test_SetDefaults_MigratesDeprecatedScheduler(t *testing.T) {
	legacy := Scheduler{
		Disabled: ptr.Bool(false),
		SchedulerConfig: SchedulerConfig{Config: config.Config{
			QueueName: "legacy-queue",
		}},
	}
	tc := &TektonConfig{Spec: TektonConfigSpec{Scheduler: legacy}}

	tc.SetDefaults(context.TODO())

	want := legacy.ToKueue()
	want.FleetManager = FleetManagerManual
	if diff := cmp.Diff(want, tc.Spec.Kueue); diff != "" {
		t.Fatalf("migrated kueue config differs (-want, +got): %s", diff)
	}
	if diff := cmp.Diff(Scheduler{}, tc.Spec.Scheduler); diff != "" {
		t.Fatalf("deprecated scheduler field was not cleared (-want, +got): %s", diff)
	}
	data, err := json.Marshal(tc.Spec)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["scheduler"]; ok {
		t.Fatalf("deprecated scheduler field remains in serialized spec: %s", data)
	}
}

func Test_SetDefaults_MigratesEnabledSchedulerWithoutQueue(t *testing.T) {
	tc := &TektonConfig{Spec: TektonConfigSpec{Scheduler: Scheduler{
		Disabled: ptr.Bool(false),
	}}}

	tc.SetDefaults(context.TODO())

	if tc.Spec.Kueue.IsDisabled() {
		t.Fatal("migrated kueue was disabled")
	}
	if tc.Spec.Kueue.QueueName != "" {
		t.Fatalf("queue name = %q, want empty", tc.Spec.Kueue.QueueName)
	}
	if diff := cmp.Diff(Scheduler{}, tc.Spec.Scheduler); diff != "" {
		t.Fatalf("deprecated scheduler field was not cleared (-want, +got): %s", diff)
	}
}

func TestMigrateSchedulerKeepsExplicitKueue(t *testing.T) {
	legacy := Scheduler{Disabled: ptr.Bool(false)}
	explicit := Kueue{Disabled: ptr.Bool(true)}
	spec := TektonConfigSpec{Scheduler: legacy, Kueue: explicit}

	if !spec.MigrateScheduler() {
		t.Fatal("expected deprecated scheduler field to be migrated")
	}
	if diff := cmp.Diff(explicit, spec.Kueue); diff != "" {
		t.Fatalf("explicit kueue config was overwritten (-want, +got): %s", diff)
	}
	if diff := cmp.Diff(Scheduler{}, spec.Scheduler); diff != "" {
		t.Fatalf("deprecated scheduler field was not cleared (-want, +got): %s", diff)
	}
}

func Test_SetDefaults_Profile(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.SetDefaults(context.TODO())
	if tc.Spec.Profile != ProfileBasic {
		t.Error("Setting default failed for TektonConfig (spec.profile)")
	}
}

func Test_SetDefaults_Pipeline_Properties(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: ProfileLite,
			Pipeline: Pipeline{
				PipelineProperties: PipelineProperties{
					SendCloudEventsForRuns: ptr.Bool(true),
				},
			},
		},
	}

	tc.SetDefaults(context.TODO())
	if *tc.Spec.Pipeline.SendCloudEventsForRuns != true {
		t.Error("Setting default failed for TektonConfig (spec.pipeline.pipelineProperties)")
	}
}

func Test_SetDefaults_Addon_Params(t *testing.T) {
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}
	t.Setenv("PLATFORM", "openshift")

	tc.SetDefaults(context.TODO())

	if len(tc.Spec.Addon.Params) != len(AddonParams) {
		t.Fatalf("Expected %d addon params, got %d", len(AddonParams), len(tc.Spec.Addon.Params))
	}
	paramsMap := ParseParams(tc.Spec.Addon.Params)

	for key, expectedValue := range AddonParams {
		value, exists := paramsMap[key]
		assert.Equal(t, true, exists, "Param %q is missing in Spec.Addon.Params", key)
		assert.Equal(t, expectedValue.Default, value, "Param %q has incorrect value", key)
	}
}

func Test_SetDefaults_Triggers_Properties(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: ProfileLite,
			Trigger: Trigger{
				TriggersProperties: TriggersProperties{
					EnableApiFields: "alpha",
				},
			},
		},
	}

	tc.SetDefaults(context.TODO())
	if tc.Spec.Trigger.EnableApiFields == "stable" {
		t.Error("Setting default failed for TektonConfig (spec.trigger.triggersProperties)")
	}
}

func Test_SetDefaults_PipelineAsCode(t *testing.T) {
	platforms := []struct {
		name      string
		getEnable func(cfg *TektonConfig) *bool
	}{
		{
			name: "openshift",
			getEnable: func(cfg *TektonConfig) *bool {
				return cfg.Spec.Platforms.OpenShift.PipelinesAsCode.Enable
			},
		},
		{
			name: "kubernetes",
			getEnable: func(cfg *TektonConfig) *bool {
				return cfg.Spec.Platforms.Kubernetes.PipelinesAsCode.Enable
			},
		},
	}

	cases := []struct {
		desc               string
		initialConfig      *TektonConfig
		wantEnable         bool
		wantPACFieldNil    bool
		skipUnlessPlatform string
	}{
		{
			desc:            "new install: nil PipelinesAsCode, no addon override",
			initialConfig:   &TektonConfig{},
			wantEnable:      true,
			wantPACFieldNil: true,
		},
		{
			desc: "disabled via addon => pipelinesAsCode.Enable=false",
			initialConfig: &TektonConfig{
				Spec: TektonConfigSpec{
					Addon: Addon{EnablePAC: ptr.Bool(false)},
				},
			},
			wantEnable:      false,
			wantPACFieldNil: true,
		},
		{
			desc: "enabled via addon => pipelinesAsCode.Enable=true",
			initialConfig: &TektonConfig{
				Spec: TektonConfigSpec{
					Addon: Addon{EnablePAC: ptr.Bool(true)},
				},
			},
			wantEnable:      true,
			wantPACFieldNil: true,
		},
		{
			desc: "existing openshift PipelinesAsCode overrides addon",
			initialConfig: &TektonConfig{
				Spec: TektonConfigSpec{
					Addon: Addon{EnablePAC: ptr.Bool(false)},
					Platforms: Platforms{
						OpenShift: &OpenShift{
							PipelinesAsCode: &PipelinesAsCode{Enable: ptr.Bool(true)},
						},
					},
				},
			},
			wantEnable:         true,
			wantPACFieldNil:    true,
			skipUnlessPlatform: "openshift",
		},
		{
			desc: "existing kubernetes PipelinesAsCode overrides addon",
			initialConfig: &TektonConfig{
				Spec: TektonConfigSpec{
					Addon: Addon{EnablePAC: ptr.Bool(false)},
					Platforms: Platforms{
						Kubernetes: &Kubernetes{
							PipelinesAsCode: &PipelinesAsCode{Enable: ptr.Bool(true)},
						},
					},
				},
			},
			wantEnable:         true,
			wantPACFieldNil:    true,
			skipUnlessPlatform: "kubernetes",
		},
	}

	for _, p := range platforms {
		t.Run(p.name, func(t *testing.T) {
			t.Setenv("PLATFORM", p.name)

			for _, c := range cases {
				if c.skipUnlessPlatform != "" && c.skipUnlessPlatform != p.name {
					continue
				}
				t.Run(c.desc, func(t *testing.T) {
					cfg := c.initialConfig.DeepCopy()
					cfg.SetDefaults(context.TODO())

					gotEnable := p.getEnable(cfg)
					if gotEnable == nil {
						t.Fatalf("PipelinesAsCode.Enable is nil for platform %q", p.name)
					}
					if *gotEnable != c.wantEnable {
						t.Errorf("for %q @ %s, Enable = %v; want %v",
							c.desc, p.name, *gotEnable, c.wantEnable)
					}

					hasPAC := cfg.Spec.Addon.EnablePAC != nil
					expectedHasPAC := !c.wantPACFieldNil
					if hasPAC != expectedHasPAC {
						t.Errorf("for %q @ %s, Addon.EnablePAC exists = %v; want exists? %v",
							c.desc, p.name, hasPAC, expectedHasPAC)
					}
				})
			}
		})
	}
}

func Test_MigrateLegacyNamespaceSyncParams(t *testing.T) {
	tests := []struct {
		name           string
		inputParams    []Param
		wantChanged    bool
		wantParams     []Param
		wantPipelineSA *bool
		wantCABundles  *bool
		wantEditRB     *bool
	}{
		{
			name:        "no legacy params present: nothing changes",
			inputParams: []Param{{Name: "other-param", Value: "x"}},
			wantChanged: false,
			wantParams:  []Param{{Name: "other-param", Value: "x"}},
		},
		{
			name: "legacy params are migrated and removed",
			inputParams: []Param{
				{Name: "createRbacResource", Value: "true"},
				{Name: "createCABundleConfigMaps", Value: "false"},
				{Name: "legacyPipelineRbac", Value: "true"},
				{Name: "unrelated-param", Value: "keep-me"},
			},
			wantChanged:    true,
			wantParams:     []Param{{Name: "unrelated-param", Value: "keep-me"}},
			wantPipelineSA: ptr.Bool(true),
			wantCABundles:  ptr.Bool(false),
			wantEditRB:     ptr.Bool(true),
		},
		{
			name: "createRbacResource=false disables SCC and edit RoleBinding too",
			inputParams: []Param{
				{Name: "createRbacResource", Value: "false"},
			},
			wantChanged:    true,
			wantParams:     []Param{},
			wantPipelineSA: ptr.Bool(false),
			wantEditRB:     ptr.Bool(false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tc := &TektonConfig{
				Spec: TektonConfigSpec{
					Params: test.inputParams,
				},
			}

			changed := MigrateLegacyNamespaceSyncParams(tc)
			if changed != test.wantChanged {
				t.Errorf("MigrateLegacyNamespaceSyncParams() changed = %v, want %v", changed, test.wantChanged)
			}
			if !cmp.Equal(tc.Spec.Params, test.wantParams) {
				t.Errorf("Spec.Params = %+v, want %+v", tc.Spec.Params, test.wantParams)
			}
			ns := tc.Spec.Platforms.OpenShift.NamespaceSync
			if test.wantPipelineSA != nil && (ns.CreatePipelineSA == nil || *ns.CreatePipelineSA != *test.wantPipelineSA) {
				t.Errorf("CreatePipelineSA = %v, want %v", ns.CreatePipelineSA, *test.wantPipelineSA)
			}
			if test.wantCABundles != nil && (ns.CreateCABundles == nil || *ns.CreateCABundles != *test.wantCABundles) {
				t.Errorf("CreateCABundles = %v, want %v", ns.CreateCABundles, *test.wantCABundles)
			}
			if test.wantEditRB != nil && (ns.CreateEditRoleBinding == nil || *ns.CreateEditRoleBinding != *test.wantEditRB) {
				t.Errorf("CreateEditRoleBinding = %v, want %v", ns.CreateEditRoleBinding, *test.wantEditRB)
			}
		})
	}
}

// Calling MigrateLegacyNamespaceSyncParams a second time (e.g. a retried
// upgrade) must be a no-op: once the legacy params are gone, "changed"
// should report false and previously-migrated typed fields must be left
// untouched rather than being reset to new defaults.
func Test_MigrateLegacyNamespaceSyncParams_Idempotent(t *testing.T) {
	tc := &TektonConfig{
		Spec: TektonConfigSpec{
			Params: []Param{{Name: "createRbacResource", Value: "false"}},
		},
	}

	if changed := MigrateLegacyNamespaceSyncParams(tc); !changed {
		t.Fatalf("expected first migration to report changed")
	}
	if changed := MigrateLegacyNamespaceSyncParams(tc); changed {
		t.Fatalf("expected second migration to be a no-op")
	}
	if ns := tc.Spec.Platforms.OpenShift.NamespaceSync; ns.CreatePipelineSA == nil || *ns.CreatePipelineSA {
		t.Errorf("expected CreatePipelineSA to remain false after second migration, got %v", ns.CreatePipelineSA)
	}
}

func Test_SetDefaults_SCC(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")

	tests := []struct {
		name        string
		inputSCC    *SCC
		expectedSCC *SCC
	}{
		{
			name:     "default SCC is set to 'pipelines-scc' when nothing is set",
			inputSCC: nil,
			expectedSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: PipelinesSCC,
			},
		},
		{
			name:     "defaulting works when default SCC is empty",
			inputSCC: &SCC{},
			expectedSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: PipelinesSCC,
			},
		},
		{
			name: "defaulting works when default not set, but maxAllowed set",
			inputSCC: &SCC{
				MaxAllowed: "coolSCC",
			},
			expectedSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: "coolSCC",
			},
		},
		{
			name: "no defaulting when default is set",
			inputSCC: &SCC{
				Default: "alreadyExistingSCC",
			},
			expectedSCC: &SCC{
				Default:    "alreadyExistingSCC",
				MaxAllowed: "alreadyExistingSCC",
			},
		},
		{
			name: "maxAllowed defaults to default SCC when only default is set",
			inputSCC: &SCC{
				Default:    "custom-scc",
				MaxAllowed: "",
			},
			expectedSCC: &SCC{
				Default:    "custom-scc",
				MaxAllowed: "custom-scc",
			},
		},
		{
			name: "maxAllowed is not overridden when explicitly set",
			inputSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: "privileged",
			},
			expectedSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: "privileged",
			},
		},
		{
			name: "empty maxAllowed gets defaulted to prevent escalation",
			inputSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: "",
			},
			expectedSCC: &SCC{
				Default:    PipelinesSCC,
				MaxAllowed: PipelinesSCC,
			},
		},
	}

	for _, test := range tests {
		tektonConfig := TektonConfig{
			Spec: TektonConfigSpec{
				Platforms: Platforms{
					OpenShift: &OpenShift{
						SCC: test.inputSCC,
					},
				},
			},
		}

		tektonConfig.SetDefaults(context.TODO())
		t.Run(test.name, func(t *testing.T) {
			if !cmp.Equal(tektonConfig.Spec.Platforms.OpenShift.SCC, test.expectedSCC) {
				t.Errorf("expected tektonconfig %#v, got %#v", test.expectedSCC, tektonConfig.Spec.Platforms.OpenShift.SCC)
			}
		})
	}
}
