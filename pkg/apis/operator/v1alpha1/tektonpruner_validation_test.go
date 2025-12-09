/*
Copyright 2025 The Tekton Authors

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

	"github.com/tektoncd/pruner/pkg/config"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func Test_ValidateTektonPruner_ValidConfig(t *testing.T) {
	disabled := false
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: TektonPrunerResourceName, // Use correct resource name
		},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: &disabled,
				TektonPrunerConfig: TektonPrunerConfig{
					GlobalConfig: &config.GlobalConfig{
						PrunerConfig: config.PrunerConfig{
							SuccessfulHistoryLimit: ptr.Int32(5),
							HistoryLimit:           ptr.Int32(10),
						},
					},
				},
			},
		},
	}

	err := tp.Validate(context.TODO())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func Test_ValidateTektonPruner_DisabledPruner(t *testing.T) {
	disabled := true
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: TektonPrunerResourceName, // Use correct resource name
		},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: &disabled,
				// Even with invalid config, validation should pass when disabled
				TektonPrunerConfig: TektonPrunerConfig{
					GlobalConfig: &config.GlobalConfig{
						PrunerConfig: config.PrunerConfig{
							SuccessfulHistoryLimit: ptr.Int32(-1), // This would be invalid if enabled
						},
					},
				},
			},
		},
	}

	err := tp.Validate(context.TODO())
	if err != nil {
		t.Errorf("Expected no error for disabled pruner, got: %v", err)
	}
}

func Test_ValidateTektonPruner_InvalidHistoryLimit(t *testing.T) {
	disabled := false
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: TektonPrunerResourceName, // Use correct resource name
		},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: &disabled,
				TektonPrunerConfig: TektonPrunerConfig{
					GlobalConfig: &config.GlobalConfig{
						PrunerConfig: config.PrunerConfig{
							SuccessfulHistoryLimit: ptr.Int32(-1), // Invalid: must be >= 0
						},
					},
				},
			},
		},
	}

	err := tp.Validate(context.TODO())
	assert.ErrorContains(t, err, "pruner config validation failed")
}

func Test_ValidateTektonPruner_InvalidResourceName(t *testing.T) {
	disabled := false
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-name", // Should be TektonPrunerResourceName
		},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: &disabled, // Must be explicitly enabled for singleton check
			},
		},
	}

	err := tp.Validate(context.TODO())
	assert.ErrorContains(t, err, "Only one instance of TektonPruner is allowed")
}

func Test_ValidateTektonPruner_InvalidResourceName_DisabledPruner(t *testing.T) {
	disabled := true
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-name", // Would normally be invalid, but allowed when disabled
		},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: &disabled, // Singleton check skipped when disabled
			},
		},
	}

	err := tp.Validate(context.TODO())
	// Should not get singleton name error when pruner is disabled
	if err != nil {
		t.Errorf("Expected no error for disabled pruner with any name, got: %v", err)
	}
}

func Test_ValidateTektonPruner_MissingTargetNamespace(t *testing.T) {
	disabled := false
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: TektonPrunerResourceName, // Use correct resource name
		},
		Spec: TektonPrunerSpec{
			// Missing TargetNamespace
			Pruner: Pruner{
				Disabled: &disabled,
			},
		},
	}

	err := tp.Validate(context.TODO())
	assert.ErrorContains(t, err, "missing field(s): spec.targetNamespace")
}

func Test_ValidateTektonPruner_ComplexValidConfig(t *testing.T) {
	disabled := false
	tp := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{
			Name: TektonPrunerResourceName, // Use correct resource name
		},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: &disabled,
				TektonPrunerConfig: TektonPrunerConfig{
					GlobalConfig: &config.GlobalConfig{
						PrunerConfig: config.PrunerConfig{
							SuccessfulHistoryLimit: ptr.Int32(5),
							FailedHistoryLimit:     ptr.Int32(3),
							HistoryLimit:           ptr.Int32(10),
						},
						Namespaces: map[string]config.NamespaceSpec{
							"dev": {
								PrunerConfig: config.PrunerConfig{
									SuccessfulHistoryLimit: ptr.Int32(3),
								},
							},
						},
					},
				},
			},
		},
	}

	err := tp.Validate(context.TODO())
	if err != nil {
		t.Errorf("Expected no error for complex valid config, got: %v", err)
	}
}
