/*
Copyright 2020 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	corev1 "k8s.io/api/core/v1"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tektoncd/operator/test/utils"
)

// AssertTektonPipelineCRReadyStatus verifies if the TektonPipeline reaches the READY status.
func AssertTektonPipelineCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonPipelineState(clients.TektonPipeline(), names.TektonPipeline,
		IsTektonPipelineReady); err != nil {
		t.Fatalf("TektonPipelineCR %q failed to get to the READY status: %v", names.TektonPipeline, err)
	}
}

// TektonPipelineCRDelete deletes tha TektonPipeline to see if all resources will be deleted
func TektonPipelineCRDelete(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if err := clients.TektonPipeline().Delete(context.TODO(), names.TektonPipeline, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonPipeline %q failed to delete: %v", names.TektonPipeline, err)
	}
	err := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		_, err := clients.TektonPipeline().Get(context.TODO(), names.TektonPipeline, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonPipeline to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonPipelineCR(clients); err != nil {
		t.Fatal(err)
	}

	// verify all but the CRD's and the Namespace are gone
	for _, u := range m.Filter(mf.NoCRDs, mf.Not(mf.ByKind("Namespace"))).Resources() {
		if _, err := m.Client.Get(&u); !apierrs.IsNotFound(err) {
			t.Fatalf("The %s %s failed to be deleted: %v", u.GetKind(), u.GetName(), err)
		}
	}
	// verify all the CRD's remain
	for _, u := range m.Filter(mf.CRDs).Resources() {
		if _, err := m.Client.Get(&u); apierrs.IsNotFound(err) {
			t.Fatalf("The %s CRD was deleted", u.GetName())
		}
	}
}

func verifyNoTektonPipelineCR(clients *utils.Clients) error {
	pipelines, err := clients.TektonPipelineAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(pipelines.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonPipeline exists")
	}
	return nil
}

// DeleteAndVerifyDeployments verify whether all the deployments for tektonpipelines are able to recreate, when they are deleted.
func DeleteAndVerifyDeployments(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	dpList, err := clients.KubeClient.Kube.AppsV1().Deployments(utils.TektonPipelineTargetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v",
			utils.TektonPipelineTargetNamespace, err)
	}
	if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found",
			utils.TektonPipelineTargetNamespace)
	}
	// Delete the first deployment and verify the operator recreates it
	deployment := dpList.Items[0]
	if err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}

	waitErr := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		dep, err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		if err != nil {
			// If the deployment is not found, we continue to wait for the availability.
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return IsDeploymentAvailable(dep)
	})

	if waitErr != nil {
		t.Fatalf("The deployment %s/%s failed to reach the desired state: %v", deployment.Namespace, deployment.Name, err)
	}

	if _, err := WaitForTektonPipelineState(clients.TektonPipeline(), names.TektonPipeline,
		IsTektonPipelineReady); err != nil {
		t.Fatalf("TektonPipelineCR %q failed to reach the desired state: %v", names.TektonPipeline, err)
	}
	t.Logf("The deployment %s/%s reached the desired state.", deployment.Namespace, deployment.Name)
}

// IsDeploymentAvailable will check the status conditions of the deployment and return true if the deployment is available.
func IsDeploymentAvailable(d *appsv1.Deployment) (bool, error) {
	return getDeploymentStatus(d) == "True", nil
}

func getDeploymentStatus(d *appsv1.Deployment) corev1.ConditionStatus {
	for _, dc := range d.Status.Conditions {
		if dc.Type == "Available" {
			return dc.Status
		}
	}
	return "unknown"
}
