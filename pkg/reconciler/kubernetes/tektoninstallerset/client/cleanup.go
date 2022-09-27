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

package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/logging"
)

var deletePropagationPolicy = v1.DeletePropagationForeground

func (i *InstallerSetClient) CleanupAll(ctx context.Context) error {
	labelSelector := labels.NewSelector()
	createdReq, _ := labels.NewRequirement(v1alpha1.CreatedByKey, selection.Equals, []string{i.resourceKind})
	if createdReq != nil {
		labelSelector = labelSelector.Add(*createdReq)
	}
	err := i.clientSet.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	return err
}

func (i *InstallerSetClient) CleanupMainSet(ctx context.Context) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", InstallerTypeMain)

	labelSelector := labels.NewSelector()
	createdReq, _ := labels.NewRequirement(v1alpha1.CreatedByKey, selection.Equals, []string{i.resourceKind})
	if createdReq != nil {
		labelSelector = labelSelector.Add(*createdReq)
	}
	typeReq, _ := labels.NewRequirement(v1alpha1.InstallerSetType, selection.Equals, []string{InstallerTypeMain})
	if typeReq != nil {
		labelSelector = labelSelector.Add(*typeReq)
	}

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}

	if len(list.Items) != 2 {
		logger.Error("found more than 2 installerSet for main, something fishy, cleaning up all")
		return i.CleanupAll(ctx)
	}

	var static, deploy *v1alpha1.TektonInstallerSet
	for i, is := range list.Items {
		if strings.Contains(is.GetName(), InstallerSubTypeStatic) {
			static = &list.Items[i]
		} else {
			deploy = &list.Items[i]
		}
	}

	// delete static installerSet first and then deployment one
	logger.Infof("deleting main-static installer set: %s", static.GetName())
	err = i.clientSet.Delete(ctx, static.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &deletePropagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete main-static installer set for %s", static.GetName())
	}

	logger.Infof("deleting main-deployment installer set: %s", deploy.GetName())
	err = i.clientSet.Delete(ctx, deploy.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &deletePropagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete main-deployment installer set for %s", deploy.GetName())
	}
	return err
}

func (i *InstallerSetClient) CleanupPreSet(ctx context.Context) error {
	return i.Cleanup(ctx, InstallerTypePre)
}

func (i *InstallerSetClient) CleanupPostSet(ctx context.Context) error {
	return i.Cleanup(ctx, InstallerTypePost)
}

func (i *InstallerSetClient) Cleanup(ctx context.Context, isType string) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	labelSelector := labels.NewSelector()
	createdReq, _ := labels.NewRequirement(v1alpha1.CreatedByKey, selection.Equals, []string{i.resourceKind})
	if createdReq != nil {
		labelSelector = labelSelector.Add(*createdReq)
	}
	typeReq, _ := labels.NewRequirement(v1alpha1.InstallerSetType, selection.Equals, []string{isType})
	if typeReq != nil {
		labelSelector = labelSelector.Add(*typeReq)
	}

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}

	if len(list.Items) != 1 {
		logger.Errorf("found more than 1 installerSet for %s something fishy, cleaning up all", isType)
		return i.CleanupAll(ctx)
	}

	logger.Infof("deleting %s installer set: %s", isType, list.Items[0].GetName())
	err = i.clientSet.Delete(ctx, list.Items[0].GetName(), metav1.DeleteOptions{
		PropagationPolicy: &deletePropagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete %s set: %s", isType, list.Items[0].GetName())
	}
	return err
}
