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

func TestTektonResult_SetDefaults(t *testing.T) {
	tests := []struct {
		name string
		Spec TektonResultSpec
		want TektonResultSpec
	}{
		{
			name: "Add TLSHostnameOverride Override",
			Spec: TektonResultSpec{
				Result: Result{
					ResultsAPIProperties: ResultsAPIProperties{
						TLSHostnameOverride: "foo.bar",
					},
				},
			},
			want: TektonResultSpec{
				Result: Result{
					ResultsAPIProperties: ResultsAPIProperties{},
				},
			},
		},
		{
			name: "Empty TLSHostnameOverride Override",
			Spec: TektonResultSpec{
				Result: Result{
					ResultsAPIProperties: ResultsAPIProperties{},
				},
			},
			want: TektonResultSpec{
				Result: Result{
					ResultsAPIProperties: ResultsAPIProperties{},
				},
			},
		},
		{
			name: "SetDefaults auto-defaults buckets when statefulset ordinals enabled",
			Spec: TektonResultSpec{
				Result: Result{
					Performance: PerformanceProperties{
						PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
							StatefulsetOrdinals: ptr.Bool(true),
						},
						Replicas: func() *int32 { r := int32(3); return &r }(),
					},
				},
			},
			want: TektonResultSpec{
				Result: Result{
					Performance: PerformanceProperties{
						PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
							StatefulsetOrdinals: ptr.Bool(true),
						},
						Replicas: func() *int32 { r := int32(3); return &r }(),
						PerformanceLeaderElectionConfig: PerformanceLeaderElectionConfig{
							Buckets: func() *uint { b := uint(3); return &b }(),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := &TektonResult{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "result",
					Namespace: "foo",
				},
				Spec: tt.Spec,
			}
			tp.SetDefaults(context.Background())
			if d := cmp.Diff(tt.want, tp.Spec); d != "" {
				t.Errorf("TektonResult SetDefaults failed: +expected,-got: %s", d)
			}

		})
	}
}

func TestResult_SetDefaultsRoutes(t *testing.T) {
	r := &Result{
		ResultsAPIProperties: ResultsAPIProperties{
			RouteEnabled: ptr.Bool(true),
		},
	}

	want := &Result{
		ResultsAPIProperties: ResultsAPIProperties{
			RouteEnabled:        ptr.Bool(true),
			RouteTLSTermination: "edge",
		},
	}

	r.setDefaults()

	if d := cmp.Diff(want, r); d != "" {
		t.Errorf("failed to set defaults %s", diff.PrintWantGot(d))
	}
}

func TestResult_SetDefaultsBucketsAutoDefaulting(t *testing.T) {
	threeReplicas := int32(3)
	threeBuckets := uint(3)

	tests := []struct {
		name string
		in   Result
		want Result
	}{
		{
			name: "statefulset-ordinals true, replicas 3, no buckets → buckets defaults to 3",
			in: Result{
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
					Replicas: &threeReplicas,
				},
			},
			want: Result{
				ResultsAPIProperties: ResultsAPIProperties{
					RouteEnabled:        ptr.Bool(IsOpenShiftPlatform()),
					RouteTLSTermination: "edge",
				},
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
					Replicas: &threeReplicas,
					PerformanceLeaderElectionConfig: PerformanceLeaderElectionConfig{
						Buckets: &threeBuckets,
					},
				},
			},
		},
		{
			name: "statefulset-ordinals false → buckets not auto-set",
			in: Result{
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(false),
					},
					Replicas: &threeReplicas,
				},
			},
			want: Result{
				ResultsAPIProperties: ResultsAPIProperties{
					RouteEnabled:        ptr.Bool(IsOpenShiftPlatform()),
					RouteTLSTermination: "edge",
				},
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(false),
					},
					Replicas: &threeReplicas,
				},
			},
		},
		{
			name: "statefulset-ordinals true, replicas 1 → buckets not auto-set",
			in: Result{
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
					Replicas: func() *int32 { r := int32(1); return &r }(),
				},
			},
			want: Result{
				ResultsAPIProperties: ResultsAPIProperties{
					RouteEnabled:        ptr.Bool(IsOpenShiftPlatform()),
					RouteTLSTermination: "edge",
				},
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
					Replicas: func() *int32 { r := int32(1); return &r }(),
				},
			},
		},
		{
			name: "statefulset-ordinals nil → buckets not auto-set",
			in: Result{
				Performance: PerformanceProperties{
					Replicas: &threeReplicas,
				},
			},
			want: Result{
				ResultsAPIProperties: ResultsAPIProperties{
					RouteEnabled:        ptr.Bool(IsOpenShiftPlatform()),
					RouteTLSTermination: "edge",
				},
				Performance: PerformanceProperties{
					Replicas: &threeReplicas,
				},
			},
		},
		{
			name: "statefulset-ordinals true, replicas nil → buckets not auto-set",
			in: Result{
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
				},
			},
			want: Result{
				ResultsAPIProperties: ResultsAPIProperties{
					RouteEnabled:        ptr.Bool(IsOpenShiftPlatform()),
					RouteTLSTermination: "edge",
				},
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
				},
			},
		},
		{
			name: "statefulset-ordinals true, replicas 3, buckets already 5 → buckets overridden to 3",
			in: Result{
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
					Replicas: &threeReplicas,
					PerformanceLeaderElectionConfig: PerformanceLeaderElectionConfig{
						Buckets: func() *uint { b := uint(5); return &b }(),
					},
				},
			},
			want: Result{
				ResultsAPIProperties: ResultsAPIProperties{
					RouteEnabled:        ptr.Bool(IsOpenShiftPlatform()),
					RouteTLSTermination: "edge",
				},
				Performance: PerformanceProperties{
					PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
						StatefulsetOrdinals: ptr.Bool(true),
					},
					Replicas: &threeReplicas,
					PerformanceLeaderElectionConfig: PerformanceLeaderElectionConfig{
						Buckets: &threeBuckets,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.setDefaults()
			if d := cmp.Diff(tt.want, tt.in); d != "" {
				t.Errorf("Result.setDefaults() buckets auto-defaulting failed: %s", diff.PrintWantGot(d))
			}
		})
	}
}
