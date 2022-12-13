/*
Copyright 2021 The Tekton Authors

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

package trigger

import (
	"context"
	"testing"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	"github.com/tektoncd/operator/pkg/client/injection/client/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestEnsureTektonTriggerExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tt := GetTektonTriggerCR(GetTektonConfig())

	// first invocation should create instance as it is non-existent and return RECONCILE_AGAIN_ERR
	_, err := EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// during second invocation instance exists but waiting on dependencies (pipeline, triggers)
	// hence returns DEPENDENCY_UPGRADE_PENDING_ERR
	_, err = EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// make upgrade checks pass
	makeUpgradeCheckPass(t, ctx, c.OperatorV1alpha1().TektonTriggers())

	// next invocation should return RECONCILE_AGAIN_ERR as Dashboard is waiting for installation (prereconcile, postreconcile, installersets...)
	_, err = EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// mark the instance ready
	markTriggersReady(t, ctx, c.OperatorV1alpha1().TektonTriggers())

	// next invocation should return nil error as the instance is ready
	_, err = EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, nil)

	// test update propagation from tektonConfig
	tt.Spec.TargetNamespace = "foobar"
	_, err = EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, nil)
}

func TestEnsureTektonTriggerCRNotExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)

	// when no instance exists, nil error is returned immediately
	err := EnsureTektonTriggerCRNotExists(ctx, c.OperatorV1alpha1().TektonTriggers())
	util.AssertEqual(t, err, nil)

	// create an instance for testing other cases
	tt := GetTektonTriggerCR(GetTektonConfig())
	_, err = EnsureTektonTriggerExists(ctx, c.OperatorV1alpha1().TektonTriggers(), tt)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when an instance exists the first invoacation should make the delete API call and
	// return RECONCILE_AGAI_ERROR. So that the deletion can be confirmed in a subsequent invocation
	err = EnsureTektonTriggerCRNotExists(ctx, c.OperatorV1alpha1().TektonTriggers())
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when the instance is completely removed from a cluster, the function should return nil error
	err = EnsureTektonTriggerCRNotExists(ctx, c.OperatorV1alpha1().TektonTriggers())
	util.AssertEqual(t, err, nil)
}

func markTriggersReady(t *testing.T, ctx context.Context, c op.TektonTriggerInterface) {
	t.Helper()
	tr, err := c.Get(ctx, v1alpha1.TriggerResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	tr.Status.MarkDependenciesInstalled()
	tr.Status.MarkPreReconcilerComplete()
	tr.Status.MarkInstallerSetAvailable()
	tr.Status.MarkInstallerSetReady()
	tr.Status.MarkPostReconcilerComplete()
	_, err = c.UpdateStatus(ctx, tr, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func makeUpgradeCheckPass(t *testing.T, ctx context.Context, c op.TektonTriggerInterface) {
	t.Helper()
	// set necessary version labels to make upgrade check pass
	trigger, err := c.Get(ctx, v1alpha1.TriggerResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	setDummyVersionLabel(t, trigger)
	_, err = c.Update(ctx, trigger, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func setDummyVersionLabel(t *testing.T, tr *v1alpha1.TektonTrigger) {
	t.Helper()

	oprVersion := "v1.2.3"
	t.Setenv(v1alpha1.VersionEnvKey, oprVersion)

	labels := tr.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = oprVersion
	tr.SetLabels(labels)
}
