package extension_test

import (
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig/extension"

	"knative.dev/pkg/client/injection/kube/client/fake"

	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestRbacCleanup(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	err := extension.RbacCleanup(ctx, c)
	util.AssertNoError(t, err)
}
