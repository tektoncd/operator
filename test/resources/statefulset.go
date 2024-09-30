/*
Copyright 2024 The Tekton Authors

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

	appsv1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tektoncd/operator/test/utils"
)

// DeleteAndVerifyStatefulSet verifies whether the Tekton Pipelines StatefulSet controller
// is able to recreate its pods after being deleted.
func DeleteAndVerifyStatefulSet(t *testing.T, clients *utils.Clients, namespace, labelSelector string) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	stsList, err := clients.KubeClient.AppsV1().StatefulSets(namespace).List(context.TODO(), listOptions)
	if err != nil {
		t.Fatalf("Failed to get any StatefulSet under the namespace %q: %v", namespace, err)
	}
	if len(stsList.Items) == 0 {
		t.Fatalf("No StatefulSet under the namespace %q was found", namespace)
	}

	// Delete the first StatefulSet and verify the operator recreates it
	statefulSet := stsList.Items[0]
	err = clients.KubeClient.AppsV1().StatefulSets(statefulSet.Namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete StatefulSet %s/%s: %v", statefulSet.Namespace, statefulSet.Name, err)
	}

	// Poll and wait for the StatefulSet to be recreated and ready
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		sts, err := clients.KubeClient.
			AppsV1().StatefulSets(statefulSet.Namespace).Get(context.TODO(), statefulSet.Name, metav1.GetOptions{})
		if err != nil {
			// If the StatefulSet is not found, we continue to wait for it to be recreated.
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// Check if the StatefulSet is available
		return IsStatefulSetAvailable(sts)
	})

	if waitErr != nil {
		t.Fatalf("The StatefulSet %s/%s failed to reach the desired state: %v", statefulSet.Namespace, statefulSet.Name, waitErr)
	}
}

// IsStatefulSetAvailable checks if a StatefulSet is available by verifying its ReadyReplicas
func IsStatefulSetAvailable(sts *appsv1.StatefulSet) (bool, error) {
	// Check if the number of ready replicas matches the desired replicas
	if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
		return true, nil
	}
	return false, nil
}
