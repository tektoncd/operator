/*
Copyright 2026 The Tekton Authors

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

package multiclusterproxyaae

import (
	"reflect"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestGetTektonMulticlusterProxyAAECR(t *testing.T) {
	t.Run("propagates options from TektonConfig to the CR spec", func(t *testing.T) {
		wantOptions := v1alpha1.AdditionalOptions{
			Deployments: map[string]appsv1.Deployment{
				"proxy-aae": {
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "proxy-aae",
										Env: []corev1.EnvVar{
											{Name: "WORKERS_SECRET_NAMESPACE", Value: "my-kueue-namespace"},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		tc := &v1alpha1.TektonConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "config"},
			Spec: v1alpha1.TektonConfigSpec{
				CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "tekton-pipelines"},
				MulticlusterProxyAAE: v1alpha1.MulticlusterProxyAAEOptions{
					Options: wantOptions,
				},
			},
		}

		cr := GetTektonMulticlusterProxyAAECR(tc, "v0.0.1")

		if cr.Spec.TargetNamespace != "tekton-pipelines" {
			t.Errorf("TargetNamespace = %q, want %q", cr.Spec.TargetNamespace, "tekton-pipelines")
		}
		if !reflect.DeepEqual(cr.Spec.Options, wantOptions) {
			t.Errorf("Options not propagated: got %+v, want %+v", cr.Spec.Options, wantOptions)
		}
	})

	t.Run("empty options when TektonConfig has no multiclusterProxyAAE field", func(t *testing.T) {
		tc := &v1alpha1.TektonConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "config"},
			Spec: v1alpha1.TektonConfigSpec{
				CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "tekton-pipelines"},
			},
		}

		cr := GetTektonMulticlusterProxyAAECR(tc, "v0.0.1")

		if !reflect.DeepEqual(cr.Spec.Options, v1alpha1.AdditionalOptions{}) {
			t.Errorf("expected empty options, got %+v", cr.Spec.Options)
		}
	})
}

func TestIsMulticlusterProxyAAEEnabled(t *testing.T) {
	tests := []struct {
		name string
		tc   *v1alpha1.TektonConfig
		want bool
	}{
		{
			name: "scheduler disabled returns false",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Scheduler: v1alpha1.Scheduler{
						Disabled: ptr.Bool(true),
					},
				},
			},
			want: false,
		},
		{
			name: "scheduler enabled but multi-cluster disabled returns false",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Scheduler: v1alpha1.Scheduler{
						Disabled: ptr.Bool(false),
						MultiClusterConfig: v1alpha1.MultiClusterConfig{
							MultiClusterDisabled: true,
							MultiClusterRole:     v1alpha1.MultiClusterRoleHub,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "scheduler enabled, multi-cluster enabled, role Spoke returns false",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Scheduler: v1alpha1.Scheduler{
						Disabled: ptr.Bool(false),
						MultiClusterConfig: v1alpha1.MultiClusterConfig{
							MultiClusterDisabled: false,
							MultiClusterRole:     v1alpha1.MultiClusterRoleSpoke,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "scheduler enabled, multi-cluster enabled, role Hub returns true",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Scheduler: v1alpha1.Scheduler{
						Disabled: ptr.Bool(false),
						MultiClusterConfig: v1alpha1.MultiClusterConfig{
							MultiClusterDisabled: false,
							MultiClusterRole:     v1alpha1.MultiClusterRoleHub,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "scheduler enabled, multi-cluster enabled, role hub (lowercase) returns true",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Scheduler: v1alpha1.Scheduler{
						Disabled: ptr.Bool(false),
						MultiClusterConfig: v1alpha1.MultiClusterConfig{
							MultiClusterDisabled: false,
							MultiClusterRole:     "hub",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMulticlusterProxyAAEEnabled(tt.tc)
			if got != tt.want {
				t.Errorf("IsMulticlusterProxyAAEEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
