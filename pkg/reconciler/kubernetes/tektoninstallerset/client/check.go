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
	"knative.dev/pkg/logging"
)

func (i *InstallerSetClient) checkSet(ctx context.Context, comp v1alpha1.TektonComponent, isType string) ([]v1alpha1.TektonInstallerSet, error) {
	logger := logging.FromContext(ctx)

	labelSelector := i.getSetLabels(isType)
	logger.Infof("%v/%v: checking installer sets with labels: %v", i.resourceKind, isType, labelSelector)

	is, err := i.clientSet.List(ctx, v1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}

	logger.Infof("%v/%v: found %v installer sets", i.resourceKind, isType, len(is.Items))

	iSets := is.Items

	if len(iSets) == 0 {
		logger.Infof("%v/%v: installer sets not found", i.resourceKind, isType)
		return nil, ErrNotFound
	}

	if len(iSets) == 1 {
		if iSets[0].DeletionTimestamp != nil {
			return iSets, ErrSetsInDeletionState
		}
	} else {
		if iSets[0].DeletionTimestamp != nil || iSets[1].DeletionTimestamp != nil {
			return iSets, ErrSetsInDeletionState
		}
	}

	switch isType {
	case InstallerTypeMain:
		if err := verifyMainInstallerSets(iSets); err != nil {
			logger.Errorf("%v/%v: failed to verify main sets: %v", i.resourceKind, isType, err)
			return iSets, err
		}
	case InstallerTypePre, InstallerTypePost:
		if len(iSets) != 1 {
			logger.Errorf("%v/%v: found multiple sets, expected one", i.resourceKind, isType)
			return iSets, ErrInvalidState
		}
	default:
		if !strings.HasPrefix(isType, InstallerTypeCustom) {
			return nil, fmt.Errorf("invalid installerSet type")
		}
		if len(iSets) != 1 {
			logger.Errorf("%v/%v: found multiple sets, expected one", i.resourceKind, isType)
			return iSets, ErrInvalidState
		}
	}

	if err := verifyMeta(i.resourceKind, isType, logger, iSets[0], comp, i.releaseVersion); err != nil {
		logger.Errorf("%v/%v: meta check failed for installer type: %v", i.resourceKind, isType, err)
		return iSets, err
	}
	logger.Infof("%v/%v: meta check passed", i.resourceKind, isType)

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

func verifyMeta(resourceKind, isType string, logger *zap.SugaredLogger, set v1alpha1.TektonInstallerSet, comp v1alpha1.TektonComponent, releaseVersion string) error {
	// Release Version Check
	logger.Infof("%v/%v: release version check", resourceKind, isType)

	rVel, ok := set.GetLabels()[v1alpha1.ReleaseVersionKey]
	if !ok {
		return ErrInvalidState
	}
	if rVel != releaseVersion {
		return ErrVersionDifferent
	}

	// Target namespace check
	logger.Infof("%v/%v: target namespace check", resourceKind, isType)

	targetNamespace, ok := set.GetAnnotations()[v1alpha1.TargetNamespaceKey]
	if !ok {
		return ErrInvalidState
	}
	if targetNamespace != comp.GetSpec().GetTargetNamespace() {
		return ErrNsDifferent
	}

	// Spec Hash Check
	logger.Infof("%v/%v: spec hash check", resourceKind, isType)

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
