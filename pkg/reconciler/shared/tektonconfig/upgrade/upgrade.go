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
	"errors"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
)

// preUpgradeMigrations is the pre-upgrade registry.
// To remove an entry: confirm DeletableFrom has shipped and every supported
// upgrade source has already applied the migration, then delete the entry here.
var preUpgradeMigrations = []Migration{
	{
		Name:          "resetTektonConfigConditions",
		Introduced:    "0.67.0",
		DeletableFrom: "0.69.0",
		Description:   "removes stale InstallSucceeded conditions from TektonConfig CR",
		Fn:            resetTektonConfigConditions,
	},
	{
		Name:          "migrateTektonSchedulerToTektonKueue",
		Introduced:    "0.82.0",
		DeletableFrom: "0.84.0",
		Description:   "migrates deprecated scheduler API / TektonConfig field to TektonKueue",
		Fn:            migrateTektonSchedulerToTektonKueue,
	},
	{
		Name:          "upgradePipelineProperties",
		Introduced:    "0.74.0",
		DeletableFrom: "0.76.0",
		Description:   "updates default enable-step-actions from false to true",
		Fn:            upgradePipelineProperties,
	},
	{
		Name:          "deleteTektonResultsTLSSecret",
		Introduced:    "0.77.0",
		DeletableFrom: "0.79.0",
		Description:   "deletes default tekton-results TLS certificate secret",
		Fn:            deleteTektonResultsTLSSecret,
	},
	{
		Name:          "preUpgradeTektonPruner",
		Introduced:    "0.77.0",
		DeletableFrom: "0.79.0",
		Description:   "migrates TektonPruner configuration from ConfigMap into TektonConfig",
		Fn:            preUpgradeTektonPruner,
	},
	{
		Name:          "removeDeprecatedDisableAffinityAssistant",
		Introduced:    "0.79.0",
		DeletableFrom: "0.81.0",
		Description:   "removes deprecated DisableAffinityAssistant from pipeline config",
		Fn:            removeDeprecatedDisableAffinityAssistant,
	},
	{
		Name:          "removeHubFromTektonConfig",
		Introduced:    "0.81.0",
		DeletableFrom: "0.83.0",
		Description:   "clears deprecated hub field from TektonConfig",
		Fn:            removeHubFromTektonConfig,
	},
	{
		Name:          "preUpgradeManualApprovalGate",
		Introduced:    "0.82.0",
		DeletableFrom: "0.84.0",
		Description:   "adopts standalone ManualApprovalGate config into TektonConfig",
		Fn:            preUpgradeManualApprovalGate,
	},
	{
		Name:          "migrateLegacyNamespaceSyncParams",
		Introduced:    "0.82.0",
		DeletableFrom: "0.84.0",
		Description:   "persists legacy namespaceSync params into typed TektonConfig fields",
		Fn:            migrateLegacyNamespaceSyncParams,
	},
	{
		Name:          "migrateResultsRouteTLSToReencrypt",
		Introduced:    "0.82.0",
		DeletableFrom: "0.84.0",
		Description:   "migrates Results route TLS termination from edge to reencrypt",
		Fn:            migrateResultsRouteTLSToReencrypt,
	},
	{
		Name:          "preUpgradePipelinesAsCodeArtifacts",
		Introduced:    "0.79.0",
		DeletableFrom: "0.81.0",
		Description:   "migrates PaC hub catalog settings to artifacthub",
		Fn:            preUpgradePipelinesAsCodeArtifacts,
	},
}

// postUpgradeMigrations is the post-upgrade registry.
// Removal rules are the same as for preUpgradeMigrations.
var postUpgradeMigrations = []Migration{
	{
		Name:          "upgradeStorageVersion",
		Introduced:    "0.67.0",
		DeletableFrom: "0.69.0",
		Description:   "performs storage version migration for operator CRDs",
		Fn:            upgradeStorageVersion,
	},
	{
		Name:          "removeClusterTaskInstallerSets",
		Introduced:    "0.74.0",
		DeletableFrom: "0.76.0",
		Description:   "removes deprecated clusterTask installersets",
		Fn:            removeClusterTaskInstallerSets,
	},
	{
		Name:          "removeVersionedTaskInstallerSets",
		Introduced:    "0.74.0",
		DeletableFrom: "0.76.0",
		Description:   "removes older versioned resolver task installersets",
		Fn:            removeVersionedTaskInstallerSets,
	},
	{
		Name:          "removeVersionedStepActionsInstallerSets",
		Introduced:    "0.74.0",
		DeletableFrom: "0.76.0",
		Description:   "removes older versioned step action resolver installersets",
		Fn:            removeVersionedStepActionsInstallerSets,
	},
}

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
	return ug.executeUpgrade(ctx, preUpgradeMigrations, true)
}

func (ug *Upgrade) RunPostUpgrade(ctx context.Context) error {
	return ug.executeUpgrade(ctx, postUpgradeMigrations, false)
}

func (ug *Upgrade) executeUpgrade(ctx context.Context, migrations []Migration, isPreUpgrade bool) error {
	// update logger
	ug.logger = logging.FromContext(ctx).Named("upgrade")

	upgradeFunctions := migrationFns(migrations)

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
	for i, _upgradeFunc := range upgradeFunctions {
		name := migrations[i].Name
		if err := _upgradeFunc(ctx, ug.logger, ug.k8sClient, ug.operatorClient, ug.restConfig); err != nil {
			if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
				ug.logger.Debugw("upgrade requested requeue", "migration", name, "error", err)
				return err
			}
			ug.logger.Errorw("error on upgrade", "migration", name, "error", err)
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
