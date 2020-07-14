package testsuites

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/setup"
	"github.com/tektoncd/operator/test/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateAutoInstall creates an instance of install.tekton.dev
// and checks whether pipelines deployments are created
func ValidateAutoInstall(t *testing.T) {
	ctx := test.NewContext(t)
	defer ctx.Cleanup()

	cr := &op.TektonPipeline{}
	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)
	helpers.ValidatePipelineSetup(t, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)

	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)
	if code := cr.Status.Conditions[0].Code; code != op.InstalledStatus {
		t.Errorf("Expected code to be %s but got %s", op.InstalledStatus, code)
	}
}

// ValidateDeploymentRecreate verifies the recreation of deployment, if it is deleted.
func ValidateDeploymentRecreate(t *testing.T) {
	ctx := test.NewContext(t)
	defer ctx.Cleanup()

	cr := &op.TektonPipeline{}
	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)

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

	helpers.ValidatePipelineSetup(t, cr, depName)
}

// ValidateDeletion ensures that deleting the cluster CR  deletes the already
// installed tekton pipeline
func ValidateDeletion(t *testing.T) {
	ctx := test.NewContext(t)
	defer ctx.Cleanup()

	cr := &op.TektonPipeline{}
	helpers.WaitForClusterCR(t, setup.ClusterCRName, cr)
	helpers.ValidatePipelineSetup(t, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)

	helpers.DeleteClusterCR(t, setup.ClusterCRName)

	helpers.ValidatePipelineCleanup(t, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)
}
