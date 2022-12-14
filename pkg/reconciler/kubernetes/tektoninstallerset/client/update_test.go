package client

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

import (
	"context"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "knative.dev/pkg/reconciler/testing"
)

func updateFilterAndTransform(extension common.Extension, updatedNs string) FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		updatedManifet, err := manifest.Transform(mf.InjectNamespace(updatedNs))
		if err != nil {
			return nil, err
		}
		return &updatedManifet, nil
	}
}

func TestInstallerSetClient_Update(t *testing.T) {
	updatedNs := "updated-ns"
	releaseVersion := "devel"
	comp := &v1alpha1.TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trigger",
		},
		Spec: v1alpha1.TektonTriggerSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "test"},
		},
	}

	expectedHash, err := hash.Compute(comp.GetSpec())
	assert.NilError(t, err)

	tests := []struct {
		name       string
		existingIS []v1alpha1.TektonInstallerSet
		resources  []unstructured.Unstructured
		setType    string
		wantIS     int
		wantErr    error
	}{
		{
			name:    "update custom set",
			setType: InstallerTypeCustom,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "addon-custom-set-asdas",
						Annotations: map[string]string{
							v1alpha1.LastAppliedHashKey: "custom",
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{
						Manifests: []unstructured.Unstructured{
							serviceAccount, deployment,
						},
					},
				},
			},
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantErr:   nil,
		},
		{
			name:    "update pre set",
			setType: InstallerTypePre,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "trigger-pre-8vue6",
						Annotations: map[string]string{
							v1alpha1.LastAppliedHashKey: "test",
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{
						Manifests: []unstructured.Unstructured{
							serviceAccount, deployment,
						},
					},
				},
			},
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantErr:   nil,
		},
		{
			name:    "update post set",
			setType: InstallerTypePost,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "trigger-post-8vue6",
						Annotations: map[string]string{
							v1alpha1.LastAppliedHashKey: "test-post",
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{
						Manifests: []unstructured.Unstructured{
							serviceAccount, deployment,
						},
					},
				},
			},
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantErr:   nil,
		},
		{
			name:    "update main sets",
			setType: InstallerTypeMain,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "trigger-main-static-8vue6",
						Annotations: map[string]string{
							v1alpha1.LastAppliedHashKey: "test-post",
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{
						Manifests: []unstructured.Unstructured{
							serviceAccount,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "trigger-main-deployment-asd34",
						Annotations: map[string]string{
							v1alpha1.LastAppliedHashKey: "test-post",
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{
						Manifests: []unstructured.Unstructured{
							deployment,
						},
					},
				},
			},
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := testing2.SetupFakeContext(t)

			runObj := []runtime.Object{}
			for i := range tt.existingIS {
				runObj = append(runObj, &tt.existingIS[i])
			}

			manifest, err := mf.ManifestFrom(mf.Slice(tt.resources))
			if err != nil {
				t.Fatalf("Failed to generate manifest: %v", err)
			}

			fakeclient := fake.NewSimpleClientset(runObj...)
			tisClient := fakeclient.OperatorV1alpha1().TektonInstallerSets()

			client := NewInstallerSetClient(tisClient, releaseVersion, "test-version", v1alpha1.KindTektonTrigger, &testMetrics{})

			updatedISs, gotErr := client.update(ctx, comp, tt.existingIS, &manifest, updateFilterAndTransform(common.NoExtension(ctx), updatedNs), tt.setType)
			if tt.wantErr != nil {
				assert.Equal(t, gotErr, tt.wantErr)
				return
			}
			assert.NilError(t, gotErr)

			// based on transformer all the resource namespace should be changed
			if tt.setType != InstallerTypeMain {
				assert.Equal(t, updatedISs[0].Spec.Manifests[0].GetNamespace(), updatedNs)
				assert.Equal(t, updatedISs[0].Spec.Manifests[1].GetNamespace(), updatedNs)
				assert.Equal(t, updatedISs[0].Annotations[v1alpha1.LastAppliedHashKey], expectedHash)
			} else {
				assert.Equal(t, updatedISs[0].Spec.Manifests[0].GetNamespace(), updatedNs)
				assert.Equal(t, updatedISs[1].Spec.Manifests[0].GetNamespace(), updatedNs)
				assert.Equal(t, updatedISs[0].Annotations[v1alpha1.LastAppliedHashKey], expectedHash)
				assert.Equal(t, updatedISs[1].Annotations[v1alpha1.LastAppliedHashKey], expectedHash)
			}
		})
	}
}
