package testgroups

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/test/helpers"
	"github.com/tektoncd/operator/test/testsuites"
)

// AddonCRD is the test group for testing addon.operator.tekton.dev CRD
func AddonCRD(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	t.Log("deploying operaot for addomn")
	err := helpers.DeployOperator(t, ctx)
	helpers.AssertNoError(t, err)
	t.Log("deployed operaot for addomn")

	t.Run("addon-install", testsuites.ValidateAddonInstall)
	t.Run("addon-delete", testsuites.ValidateAddonDeletion)
}
