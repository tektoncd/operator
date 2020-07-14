package testgroups

import (
	"testing"

	"github.com/tektoncd/operator/test/helpers"
	"github.com/tektoncd/operator/test/testsuites"
)

// AddonCRD is the test group for testing tektonaddons.operator.tekton.dev CRD
func AddonCRD(t *testing.T, clients *helpers.Clients) {
	err := helpers.DeployOperator(t, clients.KubeClient.Kube)
	helpers.AssertNoError(t, err)

	t.Run("addon-install", func(t *testing.T) {
		testsuites.ValidateAddonInstall(t, clients)
	})

	t.Run("addon-delete", func(t *testing.T) {
		testsuites.ValidateAddonDeletion(t, clients)
	})
}
