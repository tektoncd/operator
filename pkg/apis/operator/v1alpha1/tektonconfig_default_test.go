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
	if *tc.Spec.Pipeline.SendCloudEventsForRuns != true ||
		*tc.Spec.Pipeline.EnableTektonOciBundles != false {
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
	if len(tc.Spec.Addon.Params) != 3 {
		t.Error("Setting default failed for TektonConfig (spec.addon.params)")
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
	t.Setenv("PLATFORM", "openshift")

	// PAC disabled through addon
	tc := &TektonConfig{
		Spec: TektonConfigSpec{
			Addon: Addon{
				EnablePAC: ptr.Bool(false),
			},
		},
	}
	tc.SetDefaults(context.TODO())
	assert.Equal(t, *tc.Spec.Platforms.OpenShift.PipelinesAsCode.Enable, false)
	assert.Assert(t, tc.Spec.Addon.EnablePAC == nil)

	// PAC enabled through addon, moving to openshiftPipelinesAsCode
	tc = &TektonConfig{
		Spec: TektonConfigSpec{
			Addon: Addon{
				EnablePAC: ptr.Bool(true),
			},
		},
	}
	tc.SetDefaults(context.TODO())
	assert.Equal(t, *tc.Spec.Platforms.OpenShift.PipelinesAsCode.Enable, true)
	assert.Assert(t, tc.Spec.Addon.EnablePAC == nil)

	// New installation
	tc = &TektonConfig{}
	tc.SetDefaults(context.TODO())
	assert.Equal(t, *tc.Spec.Platforms.OpenShift.PipelinesAsCode.Enable, true)
	assert.Assert(t, tc.Spec.Addon.EnablePAC == nil)

	// if PAC is enabled already then ignore addon pac field
	tc = &TektonConfig{
		Spec: TektonConfigSpec{
			Addon: Addon{
				EnablePAC: ptr.Bool(false),
			},
			Platforms: Platforms{
				OpenShift: OpenShift{
					PipelinesAsCode: &PipelinesAsCode{
						Enable: ptr.Bool(true),
					},
				},
			},
		},
	}
	tc.SetDefaults(context.TODO())
	assert.Equal(t, *tc.Spec.Platforms.OpenShift.PipelinesAsCode.Enable, true)
	assert.Assert(t, tc.Spec.Addon.EnablePAC == nil)
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
