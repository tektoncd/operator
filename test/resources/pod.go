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

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func DeletePodByLabelSelector(kubeClient kubernetes.Interface, labelSelector, namespace string) error {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		err = kubeClient.CoreV1().Pods(pod.GetNamespace()).Delete(context.TODO(), pod.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func WaitForPodByLabelSelector(kubeClient kubernetes.Interface, labelSelector, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		pods, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, err
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == core.PodRunning {
				return true, nil
			}
		}
		return false, nil
	}

	return wait.PollImmediate(interval, timeout, verifyFunc)
}
