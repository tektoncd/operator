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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

var deletePropagationPolicy = v1.DeletePropagationForeground

func (i *InstallerSetClient) CleanupMainSet(ctx context.Context) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", InstallerTypeMain)

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: i.getSetLabels(InstallerTypeMain)})
	if err != nil {
		return err
	}

	if len(list.Items) != 2 {
		logger.Error("found more than 2 installerSet for main, something fishy, cleaning up all")
	}

	// delete all static installerSet first and then deployment one
	for _, is := range list.Items {
		if strings.Contains(is.GetName(), InstallerSubTypeStatic) {
			logger.Infof("deleting main-static installer set: %s", is.GetName())
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
		if strings.Contains(is.GetName(), InstallerSubTypeDeployment) {
			logger.Infof("deleting main-deployment installer set: %s", is.GetName())
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

func (i *InstallerSetClient) cleanup(ctx context.Context, isType string) error {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	list, err := i.clientSet.List(ctx, metav1.ListOptions{LabelSelector: i.getSetLabels(isType)})
	if err != nil {
		return err
	}

	if len(list.Items) != 1 {
		logger.Errorf("found more than 1 installerSet for %s something fishy, cleaning up all", isType)
	}

	for _, is := range list.Items {
		logger.Infof("deleting %s installer set: %s", isType, is.GetName())
		err = i.clientSet.Delete(ctx, is.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deletePropagationPolicy,
		})
		if err != nil {
			return fmt.Errorf("failed to delete %s set: %s", isType, is.GetName())
		}
	}
	return nil
}
