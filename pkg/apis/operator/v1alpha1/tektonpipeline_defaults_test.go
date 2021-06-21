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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func Test_SetDefaults_PipelineProperties(t *testing.T) {

	tp := &TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonPipelineSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	properties := PipelineProperties{
		DisableAffinityAssistant:                 ptr.Bool(false),
		DisableHomeEnvOverwrite:                  ptr.Bool(true),
		DisableWorkingDirectoryOverwrite:         ptr.Bool(true),
		DisableCredsInit:                         ptr.Bool(false),
		RunningInEnvironmentWithInjectedSidecars: ptr.Bool(true),
		RequireGitSshSecretKnownHosts:            ptr.Bool(false),
		EnableTektonOciBundles:                   ptr.Bool(false),
		EnableCustomTasks:                        ptr.Bool(false),
		EnableApiFields:                          PipelineApiFieldStable,
	}

	tp.SetDefaults(context.TODO())
	if !reflect.DeepEqual(tp.Spec.PipelineProperties, properties) {
		t.Error("Setting default failed for pipeline properties (spec)")
	}
}
