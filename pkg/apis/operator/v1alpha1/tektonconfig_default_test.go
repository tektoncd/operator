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
		desc            string
		initialConfig   *TektonConfig
		wantEnable      bool // expected final value of Enable
		wantPACFieldNil bool // true if we expect Addon.EnablePAC to be nil
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
			desc: "existing Platforms.PipelinesAsCode overrides addon",
			initialConfig: &TektonConfig{
				Spec: TektonConfigSpec{
					Addon: Addon{EnablePAC: ptr.Bool(false)},
					Platforms: Platforms{
						OpenShift: OpenShift{
							PipelinesAsCode: &PipelinesAsCode{Enable: ptr.Bool(true)},
						},
						Kubernetes: Kubernetes{
							PipelinesAsCode: &PipelinesAsCode{Enable: ptr.Bool(true)},
						},
					},
				},
			},
			wantEnable:      true,
			wantPACFieldNil: true,
		},
	}

	for _, p := range platforms {
		t.Run(p.name, func(t *testing.T) {
			t.Setenv("PLATFORM", p.name)

			for _, c := range cases {
				t.Run(c.desc, func(t *testing.T) {
					cfg := c.initialConfig.DeepCopy()
					cfg.SetDefaults(context.TODO())

					// 1) Check PipelinesAsCode.Enable
					gotEnable := p.getEnable(cfg)
					if gotEnable == nil {
						t.Fatalf("PipelinesAsCode.Enable is nil for platform %q", p.name)
					}
					if *gotEnable != c.wantEnable {
						t.Errorf("for %q @ %s, Enable = %v; want %v",
							c.desc, p.name, *gotEnable, c.wantEnable)
					}

					// 2) Check Addon.EnablePAC nil‑ness
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
