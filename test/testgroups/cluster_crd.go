package testgroups

import (
	"testing"

	"github.com/tektoncd/operator/test/helpers"
	"github.com/tektoncd/operator/test/testsuites"

	"github.com/operator-framework/operator-sdk/pkg/test"
)

// ClusterCRD is the test group for testing tektonpipelines.operator.tekton.dev CRD
func ClusterCRD(t *testing.T) {
	ctx := test.NewContext(t)
	defer ctx.Cleanup()

	err := helpers.DeployOperator(t, ctx)
	helpers.AssertNoError(t, err)

	t.Run("auto-installs-pipelines", testsuites.ValidateAutoInstall)
	t.Run("deployment-recreation", testsuites.ValidateDeploymentRecreate)
	t.Run("delete-pipelines", testsuites.ValidateDeletion)
}
