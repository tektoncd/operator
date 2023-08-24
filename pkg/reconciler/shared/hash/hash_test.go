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

package hash

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
)

func TestCompute(t *testing.T) {
	testHashFunc(t, Compute)
}

func TestComputeMd5(t *testing.T) {
	testHashFunc(t, ComputeMd5)
}

func testHashFunc(t *testing.T, computeFunc func(obj interface{}) (string, error)) {
	tp := &v1alpha1.TektonPipeline{
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "tekton"},
			Config: v1alpha1.Config{
				NodeSelector: map[string]string{
					"abc": "xyz",
				},
			},
		},
	}

	hash, err := computeFunc(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	// Again, calculate the hash without changing object

	hash2, err := computeFunc(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	if hash != hash2 {
		t.Fatal("hash changed without changing the object")
	}

	// Now, change the object

	tp.Spec.TargetNamespace = "changed"

	hash3, err := computeFunc(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	// Hash should be changed now
	if hash == hash3 {
		t.Fatal("hash not changed after changing the object")
	}
}
