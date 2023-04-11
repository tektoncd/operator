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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tektoncd/operator/test/utils"
)

// DeleteAndVerifyDeployments verify whether all the deployments for tektonpipelines are able to recreate, when they are deleted.
func DeleteAndVerifyDeployments(t *testing.T, clients *utils.Clients, namespace, labelSelector string) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	dpList, err := clients.KubeClient.AppsV1().Deployments(namespace).List(context.TODO(), listOptions)
	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v",
			namespace, err)
	}
	if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found", namespace)
	}
	// Delete the first deployment and verify the operator recreates it
	deployment := dpList.Items[0]
	err = clients.KubeClient.AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}

	waitErr := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		dep, err := clients.KubeClient.
			AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
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
		t.Fatalf("The deployment %s/%s failed to reach the desired state: %v", deployment.Namespace, deployment.Name, waitErr)
	}
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

func WaitForDeploymentReady(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		dep, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			return false, err
		}
		replicas := int32(1) // default replicas
		// get actual replicas count
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		isReady := replicas == dep.Status.ReadyReplicas
		return isReady, nil
	}

	return wait.PollImmediate(interval, timeout, verifyFunc)
}

func WaitForDeploymentDeletion(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}

	return wait.PollImmediate(interval, timeout, verifyFunc)
}
