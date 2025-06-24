/*
Copyright 2022 The Tekton Authors

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

package client

import (
	"context"
	"strconv"
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testing2 "knative.dev/pkg/reconciler/testing"
)

func filterAndTransform(extension common.Extension) FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		return manifest, nil
	}
}

func buildTriggerComponent(disabled bool) *v1alpha1.TektonTrigger {
	return &v1alpha1.TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{Name: "trigger"},
		Spec: v1alpha1.TektonTriggerSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "test"},
			Trigger:    v1alpha1.Trigger{Disabled: disabled},
		},
	}
}

func computeHash(comp *v1alpha1.TektonTrigger) string {
	h, err := hash.Compute(comp.GetSpec())
	if err != nil {
		panic("failed to compute hash: " + err.Error())
	}
	return h
}

func TestInstallerSetClient_Check(t *testing.T) {
	releaseVersion := "devel"

	tests := []struct {
		name      string
		resources *v1alpha1.TektonInstallerSetList
		setType   string
		wantErr   error
	}{
		{
			name:      "installer set not found",
			setType:   InstallerTypeMain,
			resources: &v1alpha1.TektonInstallerSetList{},
			wantErr:   ErrNotFound,
		},
		{
			name:    "main installer invalid state, exist only one instead of two",
			setType: InstallerTypeMain,
			resources: &v1alpha1.TektonInstallerSetList{
				Items: []v1alpha1.TektonInstallerSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc-static",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType: InstallerTypeMain,
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "some-other",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType: InstallerTypeMain,
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
				},
			},
			wantErr: ErrInvalidState,
		},
		{
			name:    "main installer set version different error",
			setType: InstallerTypeMain,
			resources: &v1alpha1.TektonInstallerSetList{
				Items: []v1alpha1.TektonInstallerSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "trigger-main-static-asd",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypeMain,
								v1alpha1.ReleaseVersionKey: "old",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "trigger-main-deployment-asd",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypeMain,
								v1alpha1.ReleaseVersionKey: "old",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc-pre",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypePre,
								v1alpha1.ReleaseVersionKey: "devel",
							},
							Annotations: map[string]string{
								v1alpha1.TargetNamespaceKey: "different-than-expected",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
				},
			},
			wantErr: ErrVersionDifferent,
		},
		{
			name:    "pre set with different namespace error",
			setType: InstallerTypePre,
			resources: &v1alpha1.TektonInstallerSetList{
				Items: []v1alpha1.TektonInstallerSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc-pre",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypePre,
								v1alpha1.ReleaseVersionKey: "devel",
							},
							Annotations: map[string]string{
								v1alpha1.TargetNamespaceKey: "different-than-expected",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
				},
			},
			wantErr: ErrNsDifferent,
		},
		{
			name:    "pre set more than one",
			setType: InstallerTypePre,
			resources: &v1alpha1.TektonInstallerSetList{
				Items: []v1alpha1.TektonInstallerSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc-pre",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypePre,
								v1alpha1.ReleaseVersionKey: "devel",
							},
							Annotations: map[string]string{
								v1alpha1.TargetNamespaceKey: "different-than-expected",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc1-pre",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypePre,
								v1alpha1.ReleaseVersionKey: "devel",
							},
							Annotations: map[string]string{
								v1alpha1.TargetNamespaceKey: "different-than-expected",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
				},
			},
			wantErr: ErrInvalidState,
		},
		{
			name:    "post set with update required error",
			setType: InstallerTypePost,
			resources: &v1alpha1.TektonInstallerSetList{
				Items: []v1alpha1.TektonInstallerSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc-post",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypePost,
								v1alpha1.ReleaseVersionKey: "devel",
							},
							Annotations: map[string]string{
								v1alpha1.TargetNamespaceKey: comp.Spec.GetTargetNamespace(),
								v1alpha1.LastAppliedHashKey: "abc",
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
				},
			},
			wantErr: ErrUpdateRequired,
		},
		{
			name:    "no error",
			setType: InstallerTypePost,
			resources: &v1alpha1.TektonInstallerSetList{
				Items: []v1alpha1.TektonInstallerSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "abc-post",
							Labels: map[string]string{
								v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
								v1alpha1.InstallerSetType:  InstallerTypePost,
								v1alpha1.ReleaseVersionKey: "devel",
							},
							Annotations: map[string]string{
								v1alpha1.TargetNamespaceKey: comp.Spec.GetTargetNamespace(),
								v1alpha1.LastAppliedHashKey: computeHash(comp),
							},
						},
						Spec: v1alpha1.TektonInstallerSetSpec{},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, disabled := range []bool{false, true} {
		t.Run("disabled="+strconv.FormatBool(disabled), func(t *testing.T) {
			comp := buildTriggerComponent(disabled)
			expectedHash := computeHash(comp)

			for _, tt := range tests {
				tt := tt
				tt.resources = tt.resources.DeepCopy()

				if tt.name == "no error" {
					tt.resources.Items[0].Annotations[v1alpha1.LastAppliedHashKey] = expectedHash
					tt.resources.Items[0].Annotations[v1alpha1.TargetNamespaceKey] = comp.Spec.GetTargetNamespace()
					tt.resources.Items[0].Labels[v1alpha1.ReleaseVersionKey] = releaseVersion
				}

				t.Run(tt.name, func(t *testing.T) {
					ctx, _ := testing2.SetupFakeContext(t)

					fakeclient := fake.NewSimpleClientset(tt.resources)
					tisClient := fakeclient.OperatorV1alpha1().TektonInstallerSets()

					client := NewInstallerSetClient(tisClient, releaseVersion, "test-version", v1alpha1.KindTektonTrigger,
						&testMetrics{})

					_, gotErr := client.checkSet(ctx, comp, tt.setType)

					if tt.wantErr != nil {
						assert.Equal(t, gotErr, tt.wantErr)
						return
					}
					assert.NilError(t, gotErr)
				})
			}
		})
	}
}
