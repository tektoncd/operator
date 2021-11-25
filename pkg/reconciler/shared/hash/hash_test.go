package hash

import (
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"testing"
)

func TestCompute(t *testing.T) {
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

	hash, err := Compute(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	// Again, calculate the hash without changing object

	hash2, err := Compute(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	if hash != hash2 {
		t.Fatal("hash changed without changing the object")
	}

	// Now, change the object

	tp.Spec.TargetNamespace = "changed"

	hash3, err := Compute(tp.Spec)
	if err != nil {
		t.Fatal("unexpected error while computing hash of obj")
	}

	// Hash should be changed now
	if hash == hash3 {
		t.Fatal("hash not changed after changing the object")
	}
}
