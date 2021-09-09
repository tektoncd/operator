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
					EnableCustomTasks: ptr.Bool(true),
				},
			},
		},
	}

	tc.SetDefaults(context.TODO())
	if *tc.Spec.Pipeline.EnableCustomTasks != true ||
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

	tc.SetDefaults(context.TODO())
	if len(tc.Spec.Addon.Params) != 2 {
		t.Error("Setting default failed for TektonConfig (spec.addon.params)")
	}
}
