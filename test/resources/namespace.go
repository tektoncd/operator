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

	"github.com/tektoncd/operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// EnsureTestNamespaceExists creates a Test Namespace
func EnsureTestNamespaceExists(clients *utils.Clients, name string) (*corev1.Namespace, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ns, err := clients.KubeClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		return clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	}
	return ns, err
}

func CreateNamespace(kubeClient kubernetes.Interface, namespace string) error {
	namespaceInstance := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := kubeClient.CoreV1().Namespaces().Create(context.TODO(), namespaceInstance, metav1.CreateOptions{})
	if err != nil && apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func DeleteNamespace(kubeClient kubernetes.Interface, namespace string) error {
	err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func DeleteNamespaceAndWait(kubeClient kubernetes.Interface, namespace string, interval, timeout time.Duration) error {
	err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	return WaitForNamespaceDeletion(kubeClient, namespace, interval, timeout)
}

func WaitForNamespaceDeletion(kubeClient kubernetes.Interface, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func() (bool, error) {
		_, err := kubeClient.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
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
