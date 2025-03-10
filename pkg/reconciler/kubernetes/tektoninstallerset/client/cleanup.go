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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/logging"
)

var deletePropagationPolicy = metav1.DeletePropagationForeground

func (i *InstallerSetClient) CleanupMainSet(ctx context.Context) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", InstallerTypeMain)

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: i.getSetLabels(InstallerTypeMain)})
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		logger.Debugf("no installerSets found for %s, nothing to clean up", InstallerTypeMain)
		return nil
	}

	if len(list.Items) != 2 {
		logger.Warnf("found %d installerSets for %s when expecting 2, proceeding with cleanup",
			len(list.Items), InstallerTypeMain)
	}

	// delete all static installerSet first and then deployment one
	for _, is := range list.Items {
		if strings.Contains(is.GetName(), InstallerSubTypeStatic) {
			logger.Debugf("deleting main-static installer set: %s", is.GetName())
			err = i.clientSet.Delete(ctx, is.GetName(), metav1.DeleteOptions{
				PropagationPolicy: &deletePropagationPolicy,
			})
			if err != nil {
				return fmt.Errorf("failed to delete main-static installer set for %s", is.GetName())
			}
		}
	}

	// now delete all deployment installerSet
	for _, is := range list.Items {
		if strings.Contains(is.GetName(), InstallerSubTypeDeployment) ||
			strings.Contains(is.GetName(), InstallerSubTypeStatefulset) {
			logger.Debugf("deleting main-deployment installer set: %s", is.GetName())
			err = i.clientSet.Delete(ctx, is.GetName(), metav1.DeleteOptions{
				PropagationPolicy: &deletePropagationPolicy,
			})
			if err != nil {
				return fmt.Errorf("failed to delete main-deployment installer set for %s", is.GetName())
			}
		}
	}
	return nil
}

func (i *InstallerSetClient) CleanupSet(ctx context.Context, setType string) error {
	return i.cleanup(ctx, setType)
}

func (i *InstallerSetClient) CleanupPreSet(ctx context.Context) error {
	return i.cleanup(ctx, InstallerTypePre)
}

func (i *InstallerSetClient) CleanupPostSet(ctx context.Context) error {
	return i.cleanup(ctx, InstallerTypePost)
}

func (i *InstallerSetClient) CleanupCustomSet(ctx context.Context, customName string) error {
	setType := InstallerTypeCustom + "-" + strings.ToLower(customName)
	return i.cleanup(ctx, setType)
}

func (i *InstallerSetClient) CleanupAllCustomSet(ctx context.Context) error {
	labelSelector := labels.NewSelector()
	createdReq, _ := labels.NewRequirement(v1alpha1.CreatedByKey, selection.Equals, []string{i.resourceKind})
	if createdReq != nil {
		labelSelector = labelSelector.Add(*createdReq)
	}
	err := i.clientSet.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to delete %s custom sets: %v", i.resourceKind, err)
	}
	return nil
}

func (i *InstallerSetClient) cleanup(ctx context.Context, isType string) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: i.getSetLabels(isType)})
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		logger.Debugf("no installerSets found for %s, nothing to clean up", isType)
		return nil
	}

	if len(list.Items) > 1 {
		logger.Warnf("found %d installerSets for %s when expecting at most 1, cleaning up all matching %s",
			len(list.Items), isType, isType)
	}

	for _, is := range list.Items {
		logger.Debugf("deleting %s installer set: %s", isType, is.GetName())
		err = i.clientSet.Delete(ctx, is.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deletePropagationPolicy,
		})
		if err != nil {
			return fmt.Errorf("failed to delete %s set: %s", isType, is.GetName())
		}
	}
	return nil
}

func (i *InstallerSetClient) CleanupSubTypeDeployment(ctx context.Context) error {
	return i.cleanupSubType(ctx, InstallerTypeMain, InstallerSubTypeDeployment)
}

func (i *InstallerSetClient) CleanupSubTypeStatefulset(ctx context.Context) error {
	return i.cleanupSubType(ctx, InstallerTypeMain, InstallerSubTypeStatefulset)
}

func (i *InstallerSetClient) cleanupSubType(ctx context.Context, isType string, isSubType string) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: i.getSetLabels(isType)})
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		logger.Debugf("no installerSets found for %s, nothing to clean up", isType)
		return nil
	}

	if len(list.Items) > 1 {
		logger.Warnf("found %d installerSets for %s when expecting at most 1, cleaning up all matching %s",
			len(list.Items), isType, isSubType)
	}

	for _, is := range list.Items {
		if strings.Contains(is.GetName(), isSubType) {
			logger.Debugf("deleting %s installer set: %s", isType, is.GetName())
			err = i.clientSet.Delete(ctx, is.GetName(), metav1.DeleteOptions{
				PropagationPolicy: &deletePropagationPolicy,
			})
			if err != nil {
				return fmt.Errorf("failed to delete %s set: %s", isType, is.GetName())
			}
		}
	}
	return nil
}

func (i *InstallerSetClient) CleanupWithLabelInstallTypeDeployment(ctx context.Context, isType string) error {
	return i.cleanupWithLabel(ctx, isType, InstallerSubTypeDeployment)
}

func (i *InstallerSetClient) CleanupWithLabelInstallTypeStatefulset(ctx context.Context, isType string) error {
	return i.cleanupWithLabel(ctx, isType, InstallerSubTypeStatefulset)
}

// cleanupWithLabel cleans installersets using isType as label selector example
// v1alpha1.InstallerSetType: chain and v1alpha1.InstallerSetInstallType: deployment
func (i *InstallerSetClient) cleanupWithLabel(ctx context.Context, isType string, isInstallType string) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: i.getSetLabelsWithTypeAndInstallType(isType, isInstallType)})
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		logger.Debugf("no installerSets found for %s, nothing to clean up", isType)
		return nil
	}

	if len(list.Items) > 1 {
		logger.Warnf("found %d installerSets for %s when expecting at most 1, cleaning up all matching %s",
			len(list.Items), isType, isInstallType)
	}

	for _, is := range list.Items {
		logger.Debugf("deleting %s installer set: %s, of installType: %s", isType, is.GetName(), isInstallType)
		err = i.clientSet.Delete(ctx, is.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deletePropagationPolicy,
		})
		if err != nil {
			return fmt.Errorf("failed to delete %s set: %s", isType, is.GetName())
		}
	}
	return nil
}
