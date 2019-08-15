package testgroups

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/test/testsuites"
)

// ClusterCRD is the test group for testing config.operator.tekton.dev CRD
func ClusterCRD(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	t.Run("auto-installs-pipelines", testsuites.ValidateAutoInstall)
	t.Run("delete-pipelines", testsuites.ValidateDeletion)
}
