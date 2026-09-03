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
	"testing"

	"github.com/google/go-cmp/cmp"

	"gotest.tools/v3/assert"
	"knative.dev/pkg/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
				Kubernetes: Kubernetes{PipelinesAsCode: kpac},
			},
		},
	}
	tc.SetDefaults(context.TODO())
	if tc.Spec.Platforms.Kubernetes.PipelinesAsCode != nil {
		t.Fatalf("expected kubernetes.pipelinesAsCode cleared after migration, got %+v", tc.Spec.Platforms.Kubernetes.PipelinesAsCode)
	}
	if tc.Spec.Platforms.OpenShift.PipelinesAsCode == nil || tc.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings["application-name"] != "test" {
		t.Fatalf("expected PAC migrated to openshift, got %+v", tc.Spec.Platforms.OpenShift.PipelinesAsCode)
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
						OpenShift: OpenShift{
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
						Kubernetes: Kubernetes{
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
				Default: PipelinesSCC,
			},
		},
		{
			name:     "defaulting works when default SCC is empty",
			inputSCC: &SCC{},
			expectedSCC: &SCC{
				Default: PipelinesSCC,
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
				Default: "alreadyExistingSCC",
			},
		},
	}

	for _, test := range tests {
		tektonConfig := TektonConfig{
			Spec: TektonConfigSpec{
				Platforms: Platforms{
					OpenShift: OpenShift{
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
