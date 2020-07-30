package helpers

import (
	"testing"

	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/setup"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/tektonpipeline"
	knativeTest "knative.dev/pkg/test"
)

type ResourceNames struct {
	TektonPipeline string
	TektonAddon    string
}

// AssertNoError confirms the error returned is nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// WaitForDeploymentDeletion checks to see if a given deployment is deleted
// the function returns an error if the given deployment is not deleted within the timeout
func WaitForDeploymentDeletion(t *testing.T, kc *knativeTest.KubeClient, namespace, name string) error {
	err := wait.Poll(tektonpipeline.APIRetry, tektonpipeline.APITimeout, func() (bool, error) {
		_, err := kc.Kube.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsGone(err) || apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		t.Logf("Waiting for deletion of %s deployment\n", name)
		return false, nil
	})
	if err == nil {
		t.Logf("%s Deployment deleted\n", name)
	}
	return err
}

func WaitForAddonCR(t *testing.T, tektonAddonClient operatorv1alpha1.TektonAddonInterface, name string) *v1alpha1.TektonAddon {
	var addon *v1alpha1.TektonAddon
	var err error
	errMessage := wait.Poll(tektonpipeline.APIRetry, tektonpipeline.APITimeout, func() (bool, error) {
		addon, err = tektonAddonClient.Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s cr\n", name)
				return false, nil
			}
			t.Logf("the error is %s\n", err.Error())
			return false, err
		}

		t.Logf("no error is found\n")
		if code := addon.Status.Conditions[0].Code; code != v1alpha1.InstalledStatus {
			t.Logf("the code is %s\n", code)
			return false, nil
		}
		return true, nil
	})
	AssertNoError(t, errMessage)
	return addon
}

func WaitForTektonPipelineCR(t *testing.T, tektonPipelineClient operatorv1alpha1.TektonPipelineInterface, name string) *v1alpha1.TektonPipeline {
	var pipeline *v1alpha1.TektonPipeline
	var err error
	errMessage := wait.Poll(tektonpipeline.APIRetry, tektonpipeline.APITimeout, func() (bool, error) {
		pipeline, err = tektonPipelineClient.Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				pipeline := &v1alpha1.TektonPipeline{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Spec: op.TektonPipelineSpec{
						TargetNamespace: setup.DefaultTargetNs,
					},
				}
				_, err = tektonPipelineClient.Create(pipeline)
				if err != nil {
					return false, err
				}
				t.Logf("Waiting for availability of TektonPipeline cr %s\n", name)
				return false, nil
			}
			return false, err
		}

		if code := pipeline.Status.Conditions[0].Code; code != v1alpha1.InstalledStatus {
			return false, nil
		}
		return true, nil
	})
	AssertNoError(t, errMessage)
	return pipeline
}

func DeletePipelineDeployment(t *testing.T, clientset *kubernetes.Clientset, dep *appsv1.Deployment) {
	err := wait.Poll(tektonpipeline.APIRetry, tektonpipeline.APITimeout, func() (bool, error) {
		err := clientset.AppsV1().Deployments(dep.Namespace).Delete(dep.Name, &metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Deletion of deployment %s failed %s \n", dep.GetName(), err)
			return false, err
		}

		return true, nil
	})

	AssertNoError(t, err)
}

func DeleteClusterCR(t *testing.T, tektonPipelineClient operatorv1alpha1.TektonPipelineInterface, name string) {
	_, err := tektonPipelineClient.Get(name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Logf("Failed to find cluster CR: %s : %s\n", name, err)
	}
	AssertNoError(t, err)

	err = wait.Poll(tektonpipeline.APIRetry, tektonpipeline.APITimeout, func() (bool, error) {
		err := tektonPipelineClient.Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Deletion of CR %s failed %s \n", name, err)
			return false, err
		}

		return true, nil
	})

	AssertNoError(t, err)
}

func ValidatePipelineSetup(t *testing.T, kc *knativeTest.KubeClient, cr *op.TektonPipeline, deployments ...string) {
	ns := cr.Spec.TargetNamespace

	for _, d := range deployments {
		err := e2eutil.WaitForDeployment(
			t, kc.Kube, ns,
			d,
			1,
			tektonpipeline.APIRetry,
			tektonpipeline.APITimeout,
		)
		AssertNoError(t, err)
	}
}

func ValidatePipelineCleanup(t *testing.T, kc *knativeTest.KubeClient, cr *op.TektonPipeline, deployments ...string) {
	ns := cr.Spec.TargetNamespace
	for _, d := range deployments {
		err := WaitForDeploymentDeletion(t, kc, ns, d)
		AssertNoError(t, err)
	}
}

func DeployOperator(t *testing.T, kubeclient kubernetes.Interface) error {
	return e2eutil.WaitForDeployment(
		t,
		kubeclient,
		tektonpipeline.TestOperatorNS,
		tektonpipeline.TestOperatorName,
		1,
		tektonpipeline.APIRetry,
		tektonpipeline.APITimeout,
	)
}
