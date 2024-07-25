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

package tektondashboard

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/pipeline/test/diff"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestTransformer(t *testing.T) {
	ctx := context.TODO()

	tektonDashboard := &v1alpha1.TektonDashboard{
		Spec: v1alpha1.TektonDashboardSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "foo-ns",
			},
			Dashboard: v1alpha1.Dashboard{
				DashboardProperties: v1alpha1.DashboardProperties{
					Readonly:     true,
					ExternalLogs: "/test",
				},
				Options: v1alpha1.AdditionalOptions{
					Deployments: map[string]appsv1.Deployment{
						"tekton-dashboard": {
							Spec: appsv1.DeploymentSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: "tekton-dashboard",
												Env: []corev1.EnvVar{
													{
														Name:  "FOO_ENV",
														Value: "foo value",
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
			Config: v1alpha1.Config{},
		},
	}

	// fetch base manifests
	targetManifest, err := common.Fetch("./testdata/test-dashboard-transformer-base.yaml")
	require.NoError(t, err)

	// fetch expected manifests
	expectedManifest, err := common.Fetch("./testdata/test-dashboard-transformer-updated.yaml")
	require.NoError(t, err)

	// update deployment image details
	t.Setenv("IMAGE_DASHBOARD_TEKTON_DASHBOARD", "foo/bar:1.0.0")

	// execute transformer of dashboard
	transformer := filterAndTransform(common.NoExtension(ctx))
	_, err = transformer(ctx, &targetManifest, tektonDashboard)
	require.NoError(t, err)

	// verify the changes
	if d := cmp.Diff(expectedManifest.Resources(), targetManifest.Resources()); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}
}
