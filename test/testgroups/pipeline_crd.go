package testgroups

import (
	"testing"

	"github.com/tektoncd/operator/test/helpers"

	"github.com/tektoncd/operator/test/testsuites"
)

// TektonPipelineCRD is the test group for testing tektonpipelines.operator.tekton.dev CRD
func TektonPipelineCRD(t *testing.T, clients *helpers.Clients) {
	t.Run("auto-installs-pipelines", func(t *testing.T) {
		testsuites.ValidateAutoInstall(t, clients)
	})

	t.Run("deployment-recreation", func(t *testing.T) {
		testsuites.ValidateDeploymentRecreate(t, clients)
	})

	t.Run("delete-pipelines", func(t *testing.T) {
		testsuites.ValidateDeletion(t, clients)
	})
}
