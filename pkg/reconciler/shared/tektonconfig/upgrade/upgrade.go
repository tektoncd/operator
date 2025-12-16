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

package upgrade

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
)

var (
	// pre upgrade functions
	preUpgradeFunctions = []upgradeFunc{
		resetTektonConfigConditions, // upgrade #1: removes conditions from TektonConfig CR, clears outdated conditions
		upgradePipelineProperties,   // upgrade #2: update default value of enable-step-actions from false to true
		// Todo: Remove the deleteTektonResultsTLSSecret upgrade function in next operator release
		deleteTektonResultsTLSSecret, // upgrade #5: deletes default tekton results tls certificate
		// TODO: Remove the preUpgradeTektonPruner upgrade function in next operator release
		preUpgradeTektonPruner,                   // upgrade #5: pre upgrade tekton pruner
		removeDeprecatedDisableAffinityAssistant, // upgrade #6: remove deprecated DisableAffinityAssistant field from pipeline config
	}

	// post upgrade functions
	postUpgradeFunctions = []upgradeFunc{
		upgradeStorageVersion,                   // upgrade #1: performs storage version migration
		removeClusterTaskInstallerSets,          // upgrade #2: removes the clusterTask installerset
		removeVersionedTaskInstallerSets,        // upgrade #3: remove the older versioned resolver task installersets
		removeVersionedStepActionsInstallerSets, // upgrade #4: remove the older versioned step action resolver installersets
	}
)

type upgradeFunc = func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error

type Upgrade struct {
	logger          *zap.SugaredLogger
	operatorVersion string
	k8sClient       kubernetes.Interface
	operatorClient  versioned.Interface
	restConfig      *rest.Config
}

func New(operatorVersion string, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) *Upgrade {
	return &Upgrade{
		k8sClient:       k8sClient,
		operatorClient:  operatorClient,
		operatorVersion: operatorVersion,
		restConfig:      restConfig,
	}
}

func (ug *Upgrade) RunPreUpgrade(ctx context.Context) error {
	return ug.executeUpgrade(ctx, preUpgradeFunctions, true)
}

func (ug *Upgrade) RunPostUpgrade(ctx context.Context) error {
	return ug.executeUpgrade(ctx, postUpgradeFunctions, false)
}

func (ug *Upgrade) executeUpgrade(ctx context.Context, upgradeFunctions []upgradeFunc, isPreUpgrade bool) error {
	// update logger
	ug.logger = logging.FromContext(ctx).Named("upgrade")

	// if upgrade not required return from here
	isUpgradeRequired, err := ug.isUpgradeRequired(ctx, isPreUpgrade)
	if err != nil {
		return err
	}
	if !isUpgradeRequired {
		return ug.markUpgradeComplete(ctx, isPreUpgrade)
	}

	if isPreUpgrade {
		if err := ug.markUpgradeFalse(ctx, isPreUpgrade, "Performing PreUpgrade", "Pre upgrade is in progress"); err != nil {
			return err
		}
		ug.logger.Debugw("executing pre upgrade functions", "numberOfFunctions", len(upgradeFunctions))
	} else {
		if err := ug.markUpgradeFalse(ctx, isPreUpgrade, "Performing PostUpgrade", "Post upgrade is in progress"); err != nil {
			return err
		}
		ug.logger.Debugw("executing post upgrade functions", "numberOfFunctions", len(upgradeFunctions))
	}

	// execute upgrade functions
	for _, _upgradeFunc := range upgradeFunctions {
		if err := _upgradeFunc(ctx, ug.logger, ug.k8sClient, ug.operatorClient, ug.restConfig); err != nil {
			ug.logger.Error("error on upgrade", err)
			return err
		}
	}
	if isPreUpgrade {
		ug.logger.Debug("completed pre upgrade execution")
	} else {
		ug.logger.Debug("completed post upgrade execution")
	}

	// update upgrade version
	return ug.updateUpgradeVersion(ctx, isPreUpgrade)
}

func (ug *Upgrade) isUpgradeRequired(ctx context.Context, isPreUpgrade bool) (bool, error) {
	tcCR, err := ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		ug.logger.Error("error on getting TektonConfig CR", err)
		return false, err
	}

	appliedUpgradeVersion := tcCR.Status.GetPostUpgradeVersion()
	if isPreUpgrade {
		appliedUpgradeVersion = tcCR.Status.GetPreUpgradeVersion()
	}

	_isUpgradeRequired := ug.operatorVersion != appliedUpgradeVersion
	return _isUpgradeRequired, nil
}

func (ug *Upgrade) updateUpgradeVersion(ctx context.Context, isPreUpgrade bool) error {
	_cr, err := ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		ug.logger.Error("error on getting TektonConfig CR", err)
		return err
	}

	// update upgrade version into TektonConfig CR, under status
	if isPreUpgrade {
		_cr.Status.SetPreUpgradeVersion(ug.operatorVersion)
	} else {
		_cr.Status.SetPostUpgradeVersion(ug.operatorVersion)
	}

	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, _cr, metav1.UpdateOptions{})
	if err != nil {
		ug.logger.Errorw("error on updating TektonConfig CR status", "version", ug.operatorVersion, err)
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func (ug *Upgrade) markUpgradeFalse(ctx context.Context, isPreUpgrade bool, reason, message string) error {
	_cr, err := ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		ug.logger.Error("error on getting TektonConfig CR", err)
		return err
	}

	isStatusChanged := false
	if isPreUpgrade {
		isStatusChanged = _cr.Status.MarkPreUpgradeFalse(reason, message)
	} else {
		isStatusChanged = _cr.Status.MarkPostUpgradeFalse(reason, message)
	}

	if isStatusChanged {
		_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, _cr, metav1.UpdateOptions{})
		if err != nil {
			ug.logger.Errorw("error on updating TektonConfig CR status", "version", ug.operatorVersion, err)
			return err
		}
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}

func (ug *Upgrade) markUpgradeComplete(ctx context.Context, isPreUpgrade bool) error {
	_cr, err := ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		ug.logger.Error("error on getting TektonConfig CR", err)
		return err
	}

	isStatusChanged := false
	if isPreUpgrade {
		isStatusChanged = _cr.Status.MarkPreUpgradeComplete()
	} else {
		isStatusChanged = _cr.Status.MarkPostUpgradeComplete()
	}

	if isStatusChanged {
		_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, _cr, metav1.UpdateOptions{})
		if err != nil {
			ug.logger.Errorw("error on updating TektonConfig CR status", "version", ug.operatorVersion, err)
			return err
		}
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}
