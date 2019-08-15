package testsuites

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/pkg/controller/config"
	"github.com/tektoncd/operator/test/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateAutoInstall creates an instance of install.tekton.dev
// and checks whether pipelines deployments are created
func ValidateAutoInstall(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	cr := helpers.WaitForClusterCR(t, config.ClusterCRName)
	helpers.ValidatePipelineSetup(t, cr,
		config.PipelineControllerName,
		config.PipelineWebhookName)
}

// ValidateDeploymentRecreate verifies the recreation of deployment, if it is deleted.
func ValidateDeploymentRecreate(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	cr := helpers.WaitForClusterCR(t, config.ClusterCRName)
	helpers.ValidatePipelineSetup(t, cr,
		config.PipelineControllerName,
		config.PipelineWebhookName)

	kc := test.Global.KubeClient
	namespace := cr.Spec.TargetNamespace
	dpList, err := kc.AppsV1().Deployments(namespace).List(metav1.ListOptions{})

	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v", namespace, err)
	}
	if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found", namespace)
	}

	// Pick up the first deployment to delete
	dep := &dpList.Items[0]
	depName := dep.GetName()
	helpers.DeletePipelineDeployment(t, dep)

	// Verify if the deleted deployment is able to recreate
	helpers.ValidatePipelineSetup(t, cr, depName)
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
