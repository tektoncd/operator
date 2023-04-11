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

func WaitForServiceAccount(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollImmediate(interval, timeout, verifyFunc)
}

func WaitForConfigMap(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollImmediate(interval, timeout, verifyFunc)
}

func WaitForRoleBinding(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollImmediate(interval, timeout, verifyFunc)
}

func WaitForClusterRole(kubeClient kubernetes.Interface, name string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollImmediate(interval, timeout, verifyFunc)
}
