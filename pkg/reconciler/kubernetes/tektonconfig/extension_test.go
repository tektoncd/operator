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

package tektonconfig

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPostReconcilePipelinesAsCode(t *testing.T) {
	enabled, disabled := true, false
	tests := []struct {
		name    string
		profile string
		enable  *bool
		wantPAC bool
	}{
		{name: "lite profile, PaC enabled", profile: v1alpha1.ProfileLite, enable: &enabled, wantPAC: true},
		{name: "basic profile, PaC enabled", profile: v1alpha1.ProfileBasic, enable: &enabled, wantPAC: true},
		{name: "basic profile, PaC disabled", profile: v1alpha1.ProfileBasic, enable: &disabled, wantPAC: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewSimpleClientset()
			oe := kubernetesExtension{operatorClientSet: client}
			config := &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName},
				Spec: v1alpha1.TektonConfigSpec{
					Profile: tt.profile,
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Platforms: v1alpha1.Platforms{
						Kubernetes: &v1alpha1.Kubernetes{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{Enable: tt.enable},
						},
					},
				},
			}

			// Creating the CR requeues until it is ready; only the CR matters here.
			_ = oe.PostReconcile(ctx, config)

			_, err := client.OperatorV1alpha1().OpenShiftPipelinesAsCodes().Get(ctx, v1alpha1.OpenShiftPipelinesAsCodeName, metav1.GetOptions{})
			if tt.wantPAC {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, apierrs.IsNotFound(err), "unexpected OpenShiftPipelinesAsCode: %v", err)
			}
		})
	}
}

// TestPostReconcileShipwrightBuild verifies that PostReconcile creates the
// ShipwrightBuild CR when it is enabled in the Kubernetes platform spec, and
// deletes it (via DeleteIfExists) when it is disabled or the block is absent.
func TestPostReconcileShipwrightBuild(t *testing.T) {
	tests := []struct {
		name        string
		shipwright  *v1alpha1.ShipwrightBuildConfig
		preexisting bool
		wantCR      bool
	}{
		{name: "enabled creates CR", shipwright: &v1alpha1.ShipwrightBuildConfig{Enabled: true}, wantCR: true},
		{name: "disabled removes existing CR", shipwright: &v1alpha1.ShipwrightBuildConfig{Enabled: false}, preexisting: true, wantCR: false},
		{name: "absent block removes existing CR", shipwright: nil, preexisting: true, wantCR: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var seed []runtime.Object
			if tt.preexisting {
				seed = append(seed, &v1alpha1.ShipwrightBuild{
					ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ShipwrightBuildResourceName},
				})
			}
			client := fake.NewSimpleClientset(seed...)
			ke := kubernetesExtension{operatorClientSet: client}
			config := &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName},
				Spec: v1alpha1.TektonConfigSpec{
					Profile: v1alpha1.ProfileBasic,
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
					Platforms: v1alpha1.Platforms{
						Kubernetes: &v1alpha1.Kubernetes{
							ShipwrightBuild: tt.shipwright,
						},
					},
				},
			}

			// Creating the CR requeues until it is ready; only the resulting CR state matters.
			_ = ke.PostReconcile(ctx, config)

			_, err := client.OperatorV1alpha1().ShipwrightBuilds().Get(ctx, v1alpha1.ShipwrightBuildResourceName, metav1.GetOptions{})
			if tt.wantCR {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, apierrs.IsNotFound(err), "unexpected ShipwrightBuild: %v", err)
			}
		})
	}
}

// TestFinalizeShipwrightBuild verifies that Finalize removes the ShipwrightBuild
// CR when it is enabled (non-All profile path).
func TestFinalizeShipwrightBuild(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(&v1alpha1.ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ShipwrightBuildResourceName},
	})
	ke := kubernetesExtension{operatorClientSet: client}
	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: v1alpha1.ProfileBasic,
			Platforms: v1alpha1.Platforms{
				Kubernetes: &v1alpha1.Kubernetes{
					ShipwrightBuild: &v1alpha1.ShipwrightBuildConfig{Enabled: true},
				},
			},
		},
	}

	// Deleting the CR requeues; only the resulting (absent) CR state matters.
	_ = ke.Finalize(ctx, config)

	_, err := client.OperatorV1alpha1().ShipwrightBuilds().Get(ctx, v1alpha1.ShipwrightBuildResourceName, metav1.GetOptions{})
	assert.Assert(t, apierrs.IsNotFound(err), "expected ShipwrightBuild to be deleted, got %v", err)
}
