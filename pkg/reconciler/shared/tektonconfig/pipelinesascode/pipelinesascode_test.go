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

package pac

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
	_, err := EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig, "v0.70.0")
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig, "v0.70.0")
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	markOPACReady(t, ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())

	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig, "v0.70.0")
	util.AssertEqual(t, err, nil)

	tConfig.Spec.TargetNamespace = "foobar"
	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig, "v0.70.0")
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig, "v0.70.0")
	util.AssertEqual(t, err, nil)
}

func TestEnsureOpenShiftPipelinesAsCodeCRNotExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)

	t.Setenv("PLATFORM", "openshift")

	err := EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())
	util.AssertEqual(t, err, nil)

	tConfig := pipeline.GetTektonConfig()
	tConfig.SetDefaults(ctx)
	_, err = EnsureOpenShiftPipelinesAsCodeExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), tConfig, "v0.70.0")
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	err = EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, c.OperatorV1alpha1().OpenShiftPipelinesAsCodes())
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

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
	opac.Status.MarkAdditionalPACControllerComplete()
	opac.Status.MarkPostReconcilerComplete()
	_, err = c.UpdateStatus(ctx, opac, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}
