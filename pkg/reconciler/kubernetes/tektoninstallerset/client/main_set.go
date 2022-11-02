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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

const (
	metricsNew     = "NewInstall"
	metricsUpgrade = "Upgrade"
)

func (i *InstallerSetClient) MainSet(ctx context.Context, comp v1alpha1.TektonComponent, manifest *mf.Manifest, filterAndTransform FilterAndTransform) error {
	logger := logging.FromContext(ctx)
	setType := InstallerTypeMain

	sets, err := i.checkSet(ctx, comp, setType)
	if err == nil {
		logger.Infof("%v/%v: found %v installer sets", i.resourceKind, setType, len(sets))
	}

	switch err {
	case ErrNotFound:
		logger.Infof("%v/%v: installer set not found, creating", i.resourceKind, setType)
		sets, err = i.create(ctx, comp, manifest, filterAndTransform, setType)
		if err != nil {
			logger.Errorf("%v/%v: failed to create main installer set: %v", i.resourceKind, setType, err)
			return err
		}
		if comp.GetStatus().GetCondition(v1alpha1.InstallerSetAvailable).IsUnknown() {
			i.metrics.LogMetrics(metricsNew, i.componentVersion, logger)
		}

	case ErrInvalidState, ErrNsDifferent, ErrVersionDifferent:
		logger.Infof("%v/%v: installer set not in valid state : %v, cleaning up!", i.resourceKind, setType, err)
		if err := i.CleanupMainSet(ctx); err != nil {
			logger.Errorf("%v/%v: failed to cleanup main installer set: %v", i.resourceKind, setType, err)
			return err
		}
		if err == ErrVersionDifferent {
			i.metrics.LogMetrics(metricsUpgrade, i.componentVersion, logger)
			markComponentStatus(comp, v1alpha1.UpgradePending)
		} else {
			markComponentStatus(comp, v1alpha1.Reinstalling)
		}
		logger.Infof("%v/%v: returning, will create main installer sets in further reconcile", i.resourceKind, setType)
		return v1alpha1.REQUEUE_EVENT_AFTER

	case ErrUpdateRequired:
		logger.Infof("%v/%v: updating installer set", i.resourceKind, setType)
		sets, err = i.update(ctx, comp, sets, manifest, filterAndTransform, setType)
		if err != nil {
			logger.Errorf("%v/%v: update failed : %v", i.resourceKind, setType, err)
			return err
		}
	case ErrSetsInDeletionState:
		logger.Infof("%v/%v: %v", i.resourceKind, setType, err)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	//Mark InstallerSet Available
	comp.GetStatus().MarkInstallerSetAvailable()

	if err := i.statusCheck(logger, setType, sets); err != nil {
		return err
	}

	//Mark InstallerSet Ready
	comp.GetStatus().MarkInstallerSetReady()

	return nil
}

func (i *InstallerSetClient) statusCheck(logger *zap.SugaredLogger, setType string, sets []v1alpha1.TektonInstallerSet) error {
	for _, set := range sets {
		ready := set.Status.GetCondition(apis.ConditionReady)
		if ready.IsUnknown() {
			logger.Infof("%v/%v: installer set %v status not set, wait !", i.resourceKind, setType, set.GetName())
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		if !ready.IsTrue() {
			msg := fmt.Sprintf("%v/%v: installer set not ready, will retry: %v", i.resourceKind, setType, ready.Message)
			logger.Infof(msg)
			return fmt.Errorf(msg)
		}
	}
	return nil
}

func markComponentStatus(comp v1alpha1.TektonComponent, status string) {
	comp.GetStatus().MarkInstallerSetNotReady(status)
	comp.GetStatus().MarkInstallerSetNotReady(status)
	comp.GetStatus().MarkPostReconcilerFailed(status)
	comp.GetStatus().MarkNotReady(status)
}
