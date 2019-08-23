package testsuites

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/setup"
	"github.com/tektoncd/operator/test/helpers"
)

// ValidateAutoInstall creates an instance of install.tekton.dev
// and checks whether pipelines deployments are created
func ValidateAutoInstall(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	cr := &op.Config{}
	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)
	helpers.ValidatePipelineSetup(t, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)

	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)
	if code := cr.Status.Conditions[0].Code; code != op.InstalledStatus {
		t.Errorf("Expected code to be %s but got %s", op.InstalledStatus, code)
	}

}

// ValidateDeletion ensures that deleting the cluster CR  deletes the already
// installed tekton pipeline
func ValidateDeletion(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	cr := &op.Config{}
	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)
	helpers.ValidatePipelineSetup(t, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)

	helpers.DeleteClusterCR(t, setup.ClusterCRName)

	helpers.ValidatePipelineCleanup(t, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)
}
