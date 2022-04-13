/*
Copyright 2020 The Tekton Authors

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

package extension

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/injection/client/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestTektonAddonCreateAndDeleteCR(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tConfig := pipeline.GetTektonConfig()
	_, err := EnsureTektonAddonExists(ctx, c.OperatorV1alpha1().TektonAddons(), tConfig)
	util.AssertNotEqual(t, err, nil)
	err = TektonAddonCRDelete(ctx, c.OperatorV1alpha1().TektonAddons(), v1alpha1.AddonResourceName)
	util.AssertEqual(t, err, nil)
}

func TestTektonDashboardUpdate(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tConfig := pipeline.GetTektonConfig()
	_, err := createAddon(ctx, c.OperatorV1alpha1().TektonAddons(), tConfig)
	util.AssertEqual(t, err, nil)
	// to update addon instance
	tConfig = &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: "all",
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines1",
			},
			Addon: v1alpha1.Addon{
				Params: []v1alpha1.Param{{
					Name:  "clusterTasks",
					Value: "false",
				}},
			},
			Config: v1alpha1.Config{
				NodeSelector: map[string]string{
					"key": "value",
				},
			},
		},
	}
	_, err = EnsureTektonAddonExists(ctx, c.OperatorV1alpha1().TektonAddons(), tConfig)
	util.AssertNotEqual(t, err, nil)
	err = TektonAddonCRDelete(ctx, c.OperatorV1alpha1().TektonAddons(), v1alpha1.AddonResourceName)
	util.AssertEqual(t, err, nil)
}

func TestTektonAddonCRDelete(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	err := TektonAddonCRDelete(ctx, c.OperatorV1alpha1().TektonAddons(), v1alpha1.AddonResourceName)
	util.AssertEqual(t, err, nil)
}
