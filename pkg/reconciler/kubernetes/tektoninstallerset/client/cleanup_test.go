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
	"strings"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testing2 "knative.dev/pkg/reconciler/testing"
)

func TestInstallerSetClient_Cleanup(t *testing.T) {
	releaseVersion := "devel"
	tests := []struct {
		name                   string
		existingIS             []v1alpha1.TektonInstallerSet
		setType                string
		afterCleanupNumberOfIS int
	}{
		{
			name:    "cleanup main",
			setType: InstallerTypeMain,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-main-static",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypeMain,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-main-deployment",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypeMain,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "trigger-pre",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypePre,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
			},
			afterCleanupNumberOfIS: 1,
		},
		{
			name:    "cleanup pre set",
			setType: InstallerTypePre,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-pre-sadsa",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypePre,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-post-asdad",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypePost,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
			},
			afterCleanupNumberOfIS: 1,
		},
		{
			name:    "cleanup post set",
			setType: InstallerTypePost,
			existingIS: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-pre-sadsa",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypePre,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-post-asdad",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     v1alpha1.KindTektonTrigger,
							v1alpha1.InstallerSetType: InstallerTypePost,
						},
					},
					Spec: v1alpha1.TektonInstallerSetSpec{},
				},
			},
			afterCleanupNumberOfIS: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := testing2.SetupFakeContext(t)

			runObj := []runtime.Object{}
			for i := range tt.existingIS {
				runObj = append(runObj, &tt.existingIS[i])
			}
			fakeClient := fake.NewSimpleClientset(runObj...)
			tisClient := fakeClient.OperatorV1alpha1().TektonInstallerSets()

			client := NewInstallerSetClient(tisClient, releaseVersion, "test-version", v1alpha1.KindTektonTrigger, &testMetrics{})

			var gotErr error
			switch tt.setType {
			case InstallerTypeMain:
				gotErr = client.CleanupMainSet(ctx)
			case InstallerTypePre:
				gotErr = client.CleanupPreSet(ctx)
			case InstallerTypePost:
				gotErr = client.CleanupPostSet(ctx)
			default:
				t.Fatal("invaid set type")
			}
			assert.NilError(t, gotErr)

			list, err := fakeClient.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Equal(t, len(list.Items), tt.afterCleanupNumberOfIS)

			if strings.Contains(list.Items[0].GetName(), tt.setType) {
				t.Fatalf("installer set %s should have been deleted but", list.Items[0].GetName())
			}
		})
	}
}
