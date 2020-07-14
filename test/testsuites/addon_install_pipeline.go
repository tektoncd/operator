package testsuites

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/setup"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/test/helpers"
	testTektonPipeline "github.com/tektoncd/operator/test/tektonpipeline"
)

// ValidateAddonInstall creates an instance of addon.operator.tekton.dev
// and checks whether dashboard deployments are created
func ValidateAddonInstall(t *testing.T, clients *helpers.Clients) {
	pipelineCR := helpers.WaitForTektonPipelineCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)
	helpers.ValidatePipelineSetup(t, clients.KubeClient, pipelineCR, setup.PipelineControllerName, setup.PipelineWebhookName)

	t.Run("creating-addon-with-version", func(t *testing.T) {
		addonCRWithVersion(t, clients)
	})

	t.Run("creating-addon-without-version", func(t *testing.T) {
		addonCRWithoutVersion(t, clients)
	})
}

// ValidateAddonDeletion ensures that deleting the addon CR  deletes the already
// installed addon pipeline
func ValidateAddonDeletion(t *testing.T, clients *helpers.Clients) {
	pipelineCR := helpers.WaitForTektonPipelineCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)
	helpers.ValidatePipelineSetup(t, clients.KubeClient, pipelineCR, setup.PipelineControllerName, setup.PipelineWebhookName)

	t.Run("deleting-addon-cr", func(t *testing.T) {
		addonCRDeletion(t, clients)
	})

	t.Run("deleting-pipeline-cr-deletes-addon", func(t *testing.T) {
		addonCRDeletionOnTektonPipelineDelete(t, clients)
	})

}

func addonCRWithVersion(t *testing.T, clients *helpers.Clients) {
	addonCR := &v1alpha1.TektonAddon{
		TypeMeta: v1.TypeMeta{
			Kind:       "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: v1alpha1.TektonAddonSpec{
			Version: "v0.1.1",
		},
	}

	verifyAddonCRDep(t, clients, addonCR)
}

func addonCRWithoutVersion(t *testing.T, clients *helpers.Clients) {
	addonCR := &v1alpha1.TektonAddon{
		TypeMeta: v1.TypeMeta{
			Kind:       "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
	}

	verifyAddonCRDep(t, clients, addonCR)
}

func deepCloneAddonCR(source *v1alpha1.TektonAddon, target *v1alpha1.TektonAddon) {
	target.APIVersion = source.APIVersion
	target.Spec.Version = source.Spec.Version
}

func verifyAddonCRDep(t *testing.T, clients *helpers.Clients, addonCR *v1alpha1.TektonAddon) {
	if existingAddonCR, err := clients.TektonAddon().Get(addonCR.Name, v1.GetOptions{}); apierrors.IsNotFound(err) {
		_, err = clients.TektonAddon().Create(addonCR)
		helpers.AssertNoError(t, err)
	} else {
		deepCloneAddonCR(addonCR, existingAddonCR)
		_, err := clients.TektonAddon().Update(existingAddonCR)
		helpers.AssertNoError(t, err)
	}

	helpers.WaitForAddonCR(t, clients.TektonAddon(), "dashboard")

	err := e2eutil.WaitForDeployment(
		t, clients.KubeClient.Kube, setup.DefaultTargetNs,
		"tekton-dashboard",
		1,
		testTektonPipeline.APIRetry,
		testTektonPipeline.APITimeout,
	)
	helpers.AssertNoError(t, err)
}

func addonCRDeletion(t *testing.T, clients *helpers.Clients) {
	addonCR := &v1alpha1.TektonAddon{
		TypeMeta: v1.TypeMeta{
			Kind:       "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
	}

	verifyAddonCRDep(t, clients, addonCR)

	err := clients.TektonAddon().Delete(addonCR.Name, &v1.DeleteOptions{})
	helpers.AssertNoError(t, err)

	err = helpers.WaitForDeploymentDeletion(t, clients.KubeClient, setup.DefaultTargetNs, "tekton-dashboard")
	helpers.AssertNoError(t, err)
}

func addonCRDeletionOnTektonPipelineDelete(t *testing.T, clients *helpers.Clients) {
	addonCR := &v1alpha1.TektonAddon{
		TypeMeta: v1.TypeMeta{
			Kind:       "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
	}

	verifyAddonCRDep(t, clients, addonCR)

	// delete the instance of tektonpipelines.operator.tekton.dev
	// this should delete the addon CR as the owner of addonCR
	// is set to the instance (name: cluster)  of tektonpipelines.operator.tekton.dev
	helpers.DeleteClusterCR(t, clients.TektonPipeline(), setup.TektonPipelineCRName)

	err := helpers.WaitForDeploymentDeletion(t, clients.KubeClient, setup.DefaultTargetNs, "tekton-dashboard")
	helpers.AssertNoError(t, err)
}
