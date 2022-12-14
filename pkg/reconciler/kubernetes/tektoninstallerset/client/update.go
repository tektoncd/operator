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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/logging"
)

func (i *InstallerSetClient) update(ctx context.Context, comp v1alpha1.TektonComponent, toBeUpdatedIS []v1alpha1.TektonInstallerSet, manifest *mf.Manifest, filterAndTransform FilterAndTransform, isType string) ([]v1alpha1.TektonInstallerSet, error) {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	if isType == InstallerTypeMain {
		sets, err := i.updateMainSets(ctx, comp, toBeUpdatedIS, manifest, filterAndTransform)
		if err != nil {
			logger.Errorf("installer set update failed for main type: %v", err)
			return sets, err
		}
		return sets, nil
	}

	logger.Infof("updating installer set: %v", toBeUpdatedIS[0].GetName())
	updatedSet, err := i.updateSet(ctx, comp, toBeUpdatedIS[0], manifest, filterAndTransform)
	if err != nil {
		return nil, fmt.Errorf("failed to update installerset : %v", err)
	}
	logger.Infof("updated installer set: %v", toBeUpdatedIS[0].GetName())
	return []v1alpha1.TektonInstallerSet{*updatedSet}, nil
}

func (i *InstallerSetClient) updateMainSets(ctx context.Context, comp v1alpha1.TektonComponent, toBeUpdatedIS []v1alpha1.TektonInstallerSet, manifest *mf.Manifest, filterAndTransform FilterAndTransform) ([]v1alpha1.TektonInstallerSet, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("updating main installersets for %v", i.resourceKind)

	staticManifest := manifest.Filter(mf.Not(mf.ByKind("Deployment")))
	deploymentManifest := manifest.Filter(mf.ByKind("Deployment"))

	var updatedSets []v1alpha1.TektonInstallerSet

	for _, is := range toBeUpdatedIS {
		logger.Infof("updating installer set: %v", is.GetName())

		var manifest *mf.Manifest
		if strings.Contains(is.GetName(), InstallerSubTypeStatic) {
			manifest = &staticManifest
		} else {
			manifest = &deploymentManifest
		}

		updatedSet, err := i.updateSet(ctx, comp, is, manifest, filterAndTransform)
		if err != nil {
			return nil, fmt.Errorf("failed to update installerset : %v", err)
		}

		logger.Infof("updated installer set: %v", is.GetName())
		updatedSets = append(updatedSets, *updatedSet)
	}
	return updatedSets, nil
}

func (i *InstallerSetClient) updateSet(ctx context.Context, comp v1alpha1.TektonComponent, set v1alpha1.TektonInstallerSet, manifest *mf.Manifest, filterAndTransform FilterAndTransform) (*v1alpha1.TektonInstallerSet, error) {
	var updatedSet *v1alpha1.TektonInstallerSet
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		onCluster, err := i.clientSet.Get(ctx, set.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		manifest, err = filterAndTransform(ctx, manifest, comp)
		if err != nil {
			return err
		}

		specHash, err := hash.Compute(comp.GetSpec())
		if err != nil {
			return err
		}

		current := onCluster.GetAnnotations()
		current[v1alpha1.LastAppliedHashKey] = specHash
		onCluster.SetAnnotations(current)

		onCluster.Spec.Manifests = manifest.Resources()

		updatedSet, err = i.clientSet.Update(ctx, onCluster, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return nil, retryErr
	}
	return updatedSet, nil
}
