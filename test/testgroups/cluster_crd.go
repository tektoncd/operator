package testgroups

import (
	"testing"

	"github.com/tektoncd/operator/test/config"
	"github.com/tektoncd/operator/test/testsuites"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

// ClusterCRD is the test group for testing config.operator.tekton.dev CRD
func ClusterCRD(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	t.Run("auto-installs-pipelines", testsuites.ValidateAutoInstall)
	t.Run("deployment-recreation", testsuites.ValidateDeploymentRecreate)
	t.Run("delete-pipelines", testsuites.ValidateDeletion)
}

func deployOperator(t *testing.T, ctx *test.TestCtx) error {
	err := ctx.InitializeClusterResources(
		&test.CleanupOptions{
			TestContext:   ctx,
			Timeout:       config.CleanupTimeout,
			RetryInterval: config.CleanupRetry,
		},
	)
	if err != nil {
		return err
	}

	namespace, err := ctx.GetNamespace()
	if err != nil {
		return err
	}

	return e2eutil.WaitForOperatorDeployment(
		t,
		test.Global.KubeClient,
		namespace,
		config.TestOperatorName,
		1,
		config.APIRetry,
		config.APITimeout,
	)
}
