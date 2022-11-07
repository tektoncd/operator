/*
Copyright 2022 The Tekton Authors

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
	"fmt"

	"github.com/tektoncd/operator/test/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func ReplaceConfigMap(kubeClient kubernetes.Interface, configMap *v1.ConfigMap) (*v1.ConfigMap, error) {
	if err := kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{}); err != nil {
		return nil, err
	}

	createdConfigMap, err := kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdConfigMap, nil
}

// WaitForTaskRunHappy polls the status of the TaskRun called name from client
// every `interval` seconds till it becomes happy with the condition function
func WaitForTaskRunHappy(client pipelinev1beta1.TektonV1beta1Interface, namespace, name string, conditionFunc func(taskRun *v1beta1.TaskRun) (bool, error)) error {
	waitErr := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		taskRun, err := client.TaskRuns(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return conditionFunc(taskRun)
	})

	if waitErr != nil {
		return fmt.Errorf("TaskRun %s is not in desired state, got: %v", name, waitErr)
	}
	return nil
}

// EnsureTaskRunExists creates a TaskRun, if it does not exist.
func EnsureTaskRunExists(client pipelinev1beta1.TektonV1beta1Interface, taskRun *v1beta1.TaskRun) (*v1beta1.TaskRun, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	tr, err := client.TaskRuns(taskRun.Namespace).Get(context.TODO(), taskRun.Name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		return client.TaskRuns(taskRun.Namespace).Create(context.TODO(), taskRun, metav1.CreateOptions{})
	}
	return tr, err
}
