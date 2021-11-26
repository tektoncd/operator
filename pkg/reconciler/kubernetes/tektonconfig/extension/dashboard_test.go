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
	ts "knative.dev/pkg/reconciler/testing"
)

func TestTektonDashboardCreateAndDeleteCR(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tConfig := pipeline.GetTektonConfig()
	err := CreateDashboardCR(ctx, tConfig, c.OperatorV1alpha1())
	util.AssertNotEqual(t, err, nil)
	err = TektonDashboardCRDelete(ctx, c.OperatorV1alpha1().TektonDashboards(), v1alpha1.DashboardResourceName)
	util.AssertEqual(t, err, nil)
}

func TestTektonDashboardCRDelete(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	err := TektonDashboardCRDelete(ctx, c.OperatorV1alpha1().TektonDashboards(), v1alpha1.DashboardResourceName)
	util.AssertEqual(t, err, nil)
}
