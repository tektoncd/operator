package testgroups

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/test/helpers"
	"github.com/tektoncd/operator/test/testsuites"
)

// AddonCRD is the test group for testing tektonaddons.operator.tekton.dev CRD
func AddonCRD(t *testing.T) {
	ctx := test.NewContext(t)
	defer ctx.Cleanup()

	t.Log("deploying operator for addon")
	err := helpers.DeployOperator(t, ctx)
	helpers.AssertNoError(t, err)
	t.Log("deployed operator for addon")

	t.Run("addon-install", testsuites.ValidateAddonInstall)
	t.Run("addon-delete", testsuites.ValidateAddonDeletion)
}
