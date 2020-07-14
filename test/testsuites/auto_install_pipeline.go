package testsuites

import (
	"testing"

	"github.com/tektoncd/operator/pkg/controller/setup"
	"github.com/tektoncd/operator/test/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateAutoInstall creates an instance of install.tekton.dev
// and checks whether pipelines deployments are created
func ValidateAutoInstall(t *testing.T, clients *helpers.Clients) {
	pipelineCR := helpers.WaitForTektonPipelineCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)
	helpers.ValidatePipelineSetup(t, clients.KubeClient, pipelineCR, setup.PipelineControllerName, setup.PipelineWebhookName)
}

// ValidateDeploymentRecreate verifies the recreation of deployment, if it is deleted.
func ValidateDeploymentRecreate(t *testing.T, clients *helpers.Clients) {
	pipelineCR := helpers.WaitForTektonPipelineCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)
	namespace := pipelineCR.Spec.TargetNamespace
	dpList, err := clients.KubeClient.Kube.AppsV1().Deployments(namespace).List(metav1.ListOptions{})

	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v", namespace, err)
	}
	if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found", namespace)
	}

	// Pick up the first deployment to delete
	dep := &dpList.Items[0]
	depName := dep.GetName()
	helpers.DeletePipelineDeployment(t, clients.KubeClient.Kube, dep)

	helpers.ValidatePipelineSetup(t, clients.KubeClient, pipelineCR, depName)
}

// ValidateDeletion ensures that deleting the cluster CR  deletes the already
// installed tekton pipeline
func ValidateDeletion(t *testing.T, clients *helpers.Clients) {
	cr := helpers.WaitForTektonPipelineCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)
	helpers.ValidatePipelineSetup(t, clients.KubeClient, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)

	helpers.DeleteClusterCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)

	helpers.ValidatePipelineCleanup(t, clients.KubeClient, cr,
		setup.PipelineControllerName,
		setup.PipelineWebhookName)
}
