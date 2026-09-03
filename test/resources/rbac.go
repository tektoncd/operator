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

	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func WaitForServiceAccount(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func(ctx context.Context) (bool, error) {
		_, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, verifyFunc)
}

func WaitForConfigMap(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func(ctx context.Context) (bool, error) {
		_, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, verifyFunc)
}

func WaitForRoleBinding(kubeClient kubernetes.Interface, name, namespace string, interval, timeout time.Duration) error {
	verifyFunc := func(ctx context.Context) (bool, error) {
		_, err := kubeClient.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, verifyFunc)
}

func WaitForClusterRole(kubeClient kubernetes.Interface, name string, interval, timeout time.Duration) error {
	verifyFunc := func(ctx context.Context) (bool, error) {
		_, err := kubeClient.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, verifyFunc)
}

// WaitForServiceAccountImagePullSecret polls until the named ServiceAccount's
// imagePullSecrets does (present=true) or does not (present=false) contain
// secretName.
func WaitForServiceAccountImagePullSecret(kubeClient kubernetes.Interface, name, namespace, secretName string, present bool, interval, timeout time.Duration) error {
	verifyFunc := func(ctx context.Context) (bool, error) {
		sa, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		found := false
		for _, ref := range sa.ImagePullSecrets {
			if ref.Name == secretName {
				found = true
				break
			}
		}
		return found == present, nil
	}
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, verifyFunc)
}

// WaitForClusterRoleBindingSubject polls until the named ClusterRoleBinding
// does (present=true) or does not (present=false) contain a ServiceAccount
// subject matching subjectName/subjectNamespace. A missing ClusterRoleBinding
// counts as "no subjects present".
func WaitForClusterRoleBindingSubject(kubeClient kubernetes.Interface, name, subjectNamespace, subjectName string, present bool, interval, timeout time.Duration) error {
	verifyFunc := func(ctx context.Context) (bool, error) {
		crb, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return !present, nil
			}
			return false, err
		}
		found := false
		for _, s := range crb.Subjects {
			if s.Kind == rbacv1.ServiceAccountKind && s.Name == subjectName && s.Namespace == subjectNamespace {
				found = true
				break
			}
		}
		return found == present, nil
	}
	return wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, verifyFunc)
}
