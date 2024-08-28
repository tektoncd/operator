/*
Copyright 2024 The Tekton Authors

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
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestSetDefaultsChain(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	cp := ChainProperties{
		ArtifactsTaskRunFormat:      "in-toto",
		ArtifactsTaskRunStorage:     ptr.String("oci"),
		ArtifactsPipelineRunFormat:  "in-toto",
		ArtifactsPipelineRunStorage: ptr.String("oci"),
		ArtifactsOCIFormat:          "simplesigning",
		ArtifactsOCIStorage:         ptr.String("oci"),
	}

	tc.SetDefaults(context.TODO())

	if d := cmp.Diff(cp, tc.Spec.ChainProperties); d != "" {
		t.Errorf("failed to set defaults %s", diff.PrintWantGot(d))
	}

}
