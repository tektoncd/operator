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

package extension

import (
	"context"
	"testing"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/injection/client/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestEnsureOpenShiftPipelinesAsCodeExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	tConfig := pipeline.GetTektonConfig()

	t.Setenv("PLATFORM", "openshift")

	tConfig.SetDefaults(ctx)
	// first invocation should create instance as it is non-existent and return RECONCILE_AGAIN_ERR
	_, err := EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// during second invocation instance exists but waiting on dependencies (pipeline, triggers)
	// hence returns DEPENDENCY_UPGRADE_PENDING_ERR
	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// mark the instance ready
	markOPACReady(t, ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())

	// next invocation should return nil error as the instance is ready
	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig)
	util.AssertEqual(t, err, nil)

	// test update propagation from tektonConfig
	tConfig.Spec.TargetNamespace = "foobar"
	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig)
	util.AssertEqual(t, err, nil)
}

func TestEnsureOpenShiftPipelinesAsCodeCRNotExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)

	t.Setenv("PLATFORM", "openshift")

	// when no instance exists, nil error is returned immediately
	err := EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())
	util.AssertEqual(t, err, nil)

	// create an instance for testing other cases
	tConfig := pipeline.GetTektonConfig()
	tConfig.SetDefaults(ctx)
	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when an instance exists the first invoacation should make the delete API call and
	// return RECONCILE_AGAI_ERROR. So that the deletion can be confirmed in a subsequent invocation
	err = EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when the instance is completely removed from a cluster, the function should return nil error
	err = EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())
	util.AssertEqual(t, err, nil)
}

func markOPACReady(t *testing.T, ctx context.Context, c op.OpenShiftPipelinesAsCodeInterface) {
	t.Helper()
	opac, err := c.Get(ctx, v1alpha1.OpenShiftPipelinesAsCodeName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	opac.Status.MarkDependenciesInstalled()
	opac.Status.MarkPreReconcilerComplete()
	opac.Status.MarkInstallerSetAvailable()
	opac.Status.MarkInstallerSetReady()
	opac.Status.MarkPostReconcilerComplete()
	_, err = c.UpdateStatus(ctx, opac, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}
