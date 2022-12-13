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
	"context"
	"testing"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/injection/client/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestEnsureTektonDashbordExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tConfig := pipeline.GetTektonConfig()

	// first invocation should create instance as it is non-existent and return RECONCILE_AGAIN_ERR
	_, err := EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// during second invocation instance exists but waiting on dependencies (pipeline, triggers)
	// hence returns RECONCILE_AGAIN_ERR
	_, err = EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// make upgrade checks pass
	makeUpgradeCheckPass(t, ctx, c.OperatorV1alpha1().TektonDashboards())

	// next invocation should return RECONCILE_AGAIN_ERR as Dashboard is waiting for installation (prereconcile, postreconcile, installersets...)
	_, err = EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// mark the instance ready
	markDashboardsReady(t, ctx, c.OperatorV1alpha1().TektonDashboards())

	// next invocation should return nil error as the instance is ready
	_, err = EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, nil)

	// test update propagation from tektonConfig
	tConfig.Spec.TargetNamespace = "foobar"
	_, err = EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, nil)
}

func TestEnsureTektonDashboardCRNotExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)

	// when no instance exists, nil error is returned immediately
	err := EnsureTektonDashboardCRNotExists(ctx, c.OperatorV1alpha1().TektonDashboards())
	util.AssertEqual(t, err, nil)

	// create an instance for testing other cases
	tConfig := pipeline.GetTektonConfig()
	_, err = EnsureTektonDashboardExists(ctx, c.OperatorV1alpha1().TektonDashboards(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when an instance exists the first invoacation should make the delete API call and
	// return RECONCILE_AGAI_ERROR. So that the deletion can be confirmed in a subsequent invocation
	err = EnsureTektonDashboardCRNotExists(ctx, c.OperatorV1alpha1().TektonDashboards())
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when the instance is completely removed from a cluster, the function should return nil error
	err = EnsureTektonDashboardCRNotExists(ctx, c.OperatorV1alpha1().TektonDashboards())
	util.AssertEqual(t, err, nil)
}

func markDashboardsReady(t *testing.T, ctx context.Context, c op.TektonDashboardInterface) {
	t.Helper()
	td, err := c.Get(ctx, v1alpha1.DashboardResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	td.Status.MarkDependenciesInstalled()
	td.Status.MarkPreReconcilerComplete()
	td.Status.MarkInstallerSetAvailable()
	td.Status.MarkInstallerSetReady()
	td.Status.MarkPostReconcilerComplete()
	_, err = c.UpdateStatus(ctx, td, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func makeUpgradeCheckPass(t *testing.T, ctx context.Context, c op.TektonDashboardInterface) {
	t.Helper()
	// set necessary version labels to make upgrade check pass
	dashboard, err := c.Get(ctx, v1alpha1.DashboardResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	setDummyVersionLabel(t, dashboard)
	_, err = c.Update(ctx, dashboard, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func setDummyVersionLabel(t *testing.T, td *v1alpha1.TektonDashboard) {
	t.Helper()

	oprVersion := "v1.2.3"
	t.Setenv(v1alpha1.VersionEnvKey, oprVersion)

	labels := td.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = oprVersion
	td.SetLabels(labels)
}
