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
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"knative.dev/pkg/logging"
)

func (i *InstallerSetClient) CheckPreSet(ctx context.Context, comp v1alpha1.TektonComponent) ([]v1alpha1.TektonInstallerSet, error) {
	return i.CheckSet(ctx, comp, InstallerTypePre)
}

func (i *InstallerSetClient) CheckMainSet(ctx context.Context, comp v1alpha1.TektonComponent) ([]v1alpha1.TektonInstallerSet, error) {
	return i.CheckSet(ctx, comp, InstallerTypeMain)
}

func (i *InstallerSetClient) CheckPostSet(ctx context.Context, comp v1alpha1.TektonComponent) ([]v1alpha1.TektonInstallerSet, error) {
	return i.CheckSet(ctx, comp, InstallerTypePost)
}

func (i *InstallerSetClient) CheckSet(ctx context.Context, comp v1alpha1.TektonComponent, isType string) ([]v1alpha1.TektonInstallerSet, error) {
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

	logger.Infof("checking installer sets with labels: %v", labelSelector.String())

	is, err := i.clientSet.List(ctx, v1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return nil, err
	}

	logger.Infof("found %v installer sets", len(is.Items))

	iSets := is.Items

	if len(iSets) == 0 {
		logger.Infof("installer sets not found")
		return nil, ErrNotFound
	}

	if len(iSets) == 1 {
		if iSets[0].DeletionTimestamp != nil {
			return nil, ErrSetsInDeletionState
		}
	} else {
		if iSets[0].DeletionTimestamp != nil || iSets[1].DeletionTimestamp != nil {
			return nil, ErrSetsInDeletionState
		}
	}

	switch isType {
	case InstallerTypeMain:
		if err := verifyMainInstallerSets(iSets); err != nil {
			logger.Errorf("failed to verify main sets: %v", err)
			return nil, err
		}
	case InstallerTypePre, InstallerTypePost:
		if len(iSets) != 1 {
			logger.Error("found multiple sets, expected one")
			return nil, ErrInvalidState
		}
	case InstallerTypeCustom:
		// TODO
	default:
		return nil, fmt.Errorf("invalid installerSet type")
	}

	if err := verifyMeta(logger, iSets[0], comp, i.releaseVersion); err != nil {
		logger.Errorf("meta check failed for installer type: %v", err)
		return nil, err
	}
	logger.Info("meta check passed")

	return iSets, nil
}

func verifyMainInstallerSets(iSets []v1alpha1.TektonInstallerSet) error {
	if len(iSets) != 2 {
		return ErrInvalidState
	}
	var static, deployment bool
	if strings.Contains(iSets[0].GetName(), InstallerSubTypeStatic) ||
		strings.Contains(iSets[1].GetName(), InstallerSubTypeStatic) {
		static = true
	}
	if strings.Contains(iSets[0].GetName(), InstallerSubTypeDeployment) ||
		strings.Contains(iSets[1].GetName(), InstallerSubTypeDeployment) {
		deployment = true
	}
	if !(static && deployment) {
		return ErrInvalidState
	}
	return nil
}

func verifyMeta(logger *zap.SugaredLogger, set v1alpha1.TektonInstallerSet, comp v1alpha1.TektonComponent, releaseVersion string) error {
	// Release Version Check
	logger.Info("release version check")

	rVel, ok := set.GetLabels()[v1alpha1.ReleaseVersionKey]
	if !ok {
		return ErrInvalidState
	}
	if rVel != releaseVersion {
		return ErrVersionDifferent
	}

	// Target namespace check
	logger.Info("target namespace check")

	targetNamespace, ok := set.GetAnnotations()[v1alpha1.TargetNamespaceKey]
	if !ok {
		return ErrInvalidState
	}
	if targetNamespace != comp.GetSpec().GetTargetNamespace() {
		return ErrNsDifferent
	}

	// Spec Hash Check
	logger.Info("spec hash check")

	expectedHash, err := hash.Compute(comp.GetSpec())
	if err != nil {
		return err
	}
	onClusterHash, ok := set.GetAnnotations()[v1alpha1.LastAppliedHashKey]
	if !ok {
		return ErrInvalidState
	}
	if onClusterHash != expectedHash {
		return ErrUpdateRequired
	}

	return nil
}
