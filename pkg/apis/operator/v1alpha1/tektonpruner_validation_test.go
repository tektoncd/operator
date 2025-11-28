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
	"strings"
	"testing"

	"github.com/tektoncd/pruner/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

func TestTektonPruner_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pruner  *TektonPruner
		wantErr bool
	}{
		{
			name: "valid tektonpruner with global config",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{
					Name: TektonPrunerResourceName,
				},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									TTLSecondsAfterFinished: ptr.Int32(3600),
									SuccessfulHistoryLimit:  ptr.Int32(50),
									FailedHistoryLimit:      ptr.Int32(20),
									HistoryLimit:            ptr.Int32(100),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name - only 'pruner' is allowed",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-name"},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid ttlSecondsAfterFinished - negative value",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									TTLSecondsAfterFinished: ptr.Int32(-100),
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid successfulHistoryLimit - negative value",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									SuccessfulHistoryLimit: ptr.Int32(-50),
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid failedHistoryLimit - negative value",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									FailedHistoryLimit: ptr.Int32(-20),
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid historyLimit - negative value",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									HistoryLimit: ptr.Int32(-100),
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid enforcedConfigLevel - invalid value",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									EnforcedConfigLevel: (*config.EnforcedConfigLevel)(ptr.String("invalid")),
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid enforcedConfigLevel - global",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									EnforcedConfigLevel:     (*config.EnforcedConfigLevel)(ptr.String("global")),
									TTLSecondsAfterFinished: ptr.Int32(3600),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid - disabled pruner (no validation required)",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid - namespace-level config with selectors in global config (should fail)",
			pruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									TTLSecondsAfterFinished: ptr.Int32(3600),
								},
								Namespaces: map[string]config.NamespaceSpec{
									"test-namespace": {
										PrunerConfig: config.PrunerConfig{
											TTLSecondsAfterFinished: ptr.Int32(1800),
										},
										PipelineRuns: []config.ResourceSpec{
											{
												Name: "my-pipeline",
												Selector: []config.SelectorSpec{
													{
														MatchLabels: map[string]string{
															"app": "test",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true, // Selectors are not allowed in global ConfigMap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pruner.Validate(context.TODO())
			if (err != nil) != tt.wantErr {
				t.Errorf("TektonPruner.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTektonPruner_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name       string
		oldPruner  *TektonPruner
		newPruner  *TektonPruner
		wantErr    bool
		errMessage string
	}{
		{
			name: "valid update - same targetNamespace",
			oldPruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
					},
				},
			},
			newPruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
						TektonPrunerConfig: TektonPrunerConfig{
							GlobalConfig: config.GlobalConfig{
								PrunerConfig: config.PrunerConfig{
									TTLSecondsAfterFinished: ptr.Int32(7200),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid update - changing targetNamespace",
			oldPruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
					},
				},
			},
			newPruner: &TektonPruner{
				ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
				Spec: TektonPrunerSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "openshift-pipelines",
					},
					Pruner: Pruner{
						Disabled: ptr.Bool(false),
					},
				},
			},
			wantErr:    true,
			errMessage: "targetNamespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := apis.WithinUpdate(context.Background(), tt.oldPruner)
			err := tt.newPruner.Validate(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("TektonPruner.Validate() on update error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMessage != "" {
				if !containsString(err.Error(), tt.errMessage) {
					t.Errorf("TektonPruner.Validate() error message = %v, want to contain %v", err.Error(), tt.errMessage)
				}
			}
		})
	}
}

func TestTektonPruner_ValidateDelete(t *testing.T) {
	pruner := &TektonPruner{
		ObjectMeta: metav1.ObjectMeta{Name: TektonPrunerResourceName},
		Spec: TektonPrunerSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Pruner: Pruner{
				Disabled: ptr.Bool(false),
			},
		},
	}

	err := pruner.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("TektonPruner.Validate() on Delete expected no error, but got one: %v", err)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
