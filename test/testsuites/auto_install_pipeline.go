package testsuites

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/config"
	"github.com/tektoncd/operator/test/helpers"
)

// ValidateAutoInstall creates an instance of install.tekton.dev
// and checks whether openshift pipelines deployment are created
func ValidateAutoInstall(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	cr := helpers.WaitForClusterCR(t, config.ClusterCRName)
	helpers.ValidatePipelineSetup(t, cr,
		config.PipelineControllerName,
		config.PipelineWebhookName)

	cr = helpers.WaitForClusterCR(t, config.ClusterCRName)
	if code := cr.Status.Conditions[0].Code; code != op.InstalledStatus {
		t.Errorf("Expected code to be %s but got %s", op.InstalledStatus, code)
	}

}

// ValidateDeletion ensures that deleting the cluster CR  deletes the already
// installed tekton pipeline
func ValidateDeletion(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	cr := helpers.WaitForClusterCR(t, config.ClusterCRName)
	helpers.ValidatePipelineSetup(t, cr,
		config.PipelineControllerName,
		config.PipelineWebhookName)

	helpers.DeleteClusterCR(t, config.ClusterCRName)

	helpers.ValidatePipelineCleanup(t, cr,
		config.PipelineControllerName,
		config.PipelineWebhookName)
}
