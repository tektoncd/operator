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

package common

import (
	"context"
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	labelKeyTargetNamespace = "operator.tekton.dev/targetNamespace"
)

func ReconcileTargetNamespace(ctx context.Context, labels map[string]string, tektonComponent v1alpha1.TektonComponent, kubeClientSet kubernetes.Interface) error {
	// get logger
	logger := logging.FromContext(ctx)

	logger.Debugw("reconciling target namespace",
		"targetNamespace", tektonComponent.GetSpec().GetTargetNamespace(),
	)

	// ensure only one namespace with the specified targetNamespace label
	nsList, err := kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", labelKeyTargetNamespace)})
	if err != nil {
		logger.Errorw("error on listing namespaces",
			"targetNamespace", tektonComponent.GetSpec().GetTargetNamespace(),
			err,
		)
		return err
	}

	var targetNamespace *corev1.Namespace
	namespaceDeletionInProgress := false
	for _, namespace := range nsList.Items {
		if namespace.Name == tektonComponent.GetSpec().GetTargetNamespace() && namespace.DeletionTimestamp == nil {
			_targetNamespace := namespace.DeepCopy()
			targetNamespace = _targetNamespace
		} else {
			// delete irrelevant namespaces
			// if deletionTimestamp is not nil, that indicates, the namespace is in deletion state
			if namespace.DeletionTimestamp == nil {
				if err := kubeClientSet.CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{}); err != nil {
					logger.Errorw("error on deleting a namespace",
						"namespace", namespace.Name,
						err,
					)
					return err
				}
			} else {
				logger.Infof("'%v' namespace is in deletion state", namespace.Name)
				namespaceDeletionInProgress = true
			}
		}
	}

	// if some of the namespaces are in deletion state, requeue and try again on next reconcile cycle
	if namespaceDeletionInProgress {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// verify the target namespace exists, now get with targetNamespace name
	if targetNamespace == nil {
		_targetNamespace, err := kubeClientSet.CoreV1().Namespaces().Get(ctx, tektonComponent.GetSpec().GetTargetNamespace(), metav1.GetOptions{})
		if err == nil {
			if _targetNamespace.DeletionTimestamp != nil {
				logger.Infof("'%v' namespace is in deletion state", tektonComponent.GetSpec().GetTargetNamespace())
				return v1alpha1.REQUEUE_EVENT_AFTER
			}
			targetNamespace = _targetNamespace
		} else if !errors.IsNotFound(err) {
			return err
		}
	}

	// owner reference used for target namespace
	ownerRef := *metav1.NewControllerRef(tektonComponent, tektonComponent.GroupVersionKind())

	// update required labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[labelKeyTargetNamespace] = "true" // include target namespace label

	// if a namespace found, update the required fields
	if targetNamespace != nil {
		// verify the existing namespace has the required fields, if not update
		updateRequired := false

		// update owner reference, if no one is owned
		if len(targetNamespace.GetOwnerReferences()) == 0 {
			targetNamespace.OwnerReferences = []metav1.OwnerReference{ownerRef}
			updateRequired = true
		}

		// update labels
		for expectedLabelKey, expectedLabelValue := range labels {
			expectedLabelFound := false
			for actualLabelKey, actualLabelValue := range targetNamespace.GetLabels() {
				if expectedLabelKey == actualLabelKey && expectedLabelValue == actualLabelValue {
					expectedLabelFound = true
					break
				}
			}
			// update label if not found
			if !expectedLabelFound {
				if targetNamespace.Labels == nil {
					targetNamespace.Labels = map[string]string{}
				}
				targetNamespace.Labels[expectedLabelKey] = expectedLabelValue
				updateRequired = true
			}
		}

		// update the namespace, if required
		if updateRequired {
			_, err = kubeClientSet.CoreV1().Namespaces().Update(ctx, targetNamespace, metav1.UpdateOptions{})
			if err != nil {
				logger.Errorw("error on updating target namespace",
					"targetNamespace", tektonComponent.GetSpec().GetTargetNamespace(),
					err,
				)
			}
			return err
		}

	} else {
		// create target namespace
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:            tektonComponent.GetSpec().GetTargetNamespace(),
				Labels:          labels,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
		}

		if _, err := kubeClientSet.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{}); err != nil {
			logger.Errorw("error on creating target namespace",
				"targetNamespace", tektonComponent.GetSpec().GetTargetNamespace(),
				err,
			)
			return err
		}
	}

	return nil
}
