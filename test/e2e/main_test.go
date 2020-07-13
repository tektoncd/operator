// +build e2e

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

package e2e

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/pkg/apis"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/testgroups"
	_ "github.com/tektoncd/plumbing/scripts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	test.MainEntry(m)
}

func TestPipelineOperator(t *testing.T) {
	initTestingFramework(t)

	// Run test groups (test each CRDs)
	t.Run("pipeline-crd", testgroups.ClusterCRD)
	t.Run("addon-crd", testgroups.AddonCRD)
}

func initTestingFramework(t *testing.T) {
	apiVersion := "operator.tekton.dev/v1alpha1"
	kind := "TektonPipeline"

	tektonPipelineList := &op.TektonPipelineList{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
	}

	if err := test.AddToFrameworkScheme(apis.AddToScheme, tektonPipelineList); err != nil {
		t.Fatalf("failed to add '%s %s': %v", apiVersion, kind, err)
	}
}
