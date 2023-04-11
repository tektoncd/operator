/*
Copyright 2023 The Tekton Authors

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
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func WaitForJobCompletion(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		job, err := kubeClient.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			return false, err
		}
		isCompleted := job.Status.Succeeded > 0
		return isCompleted, nil
	}

	return wait.PollImmediate(interval, timeout, verifyFunc)
}

func WaitForJobDeletion(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
