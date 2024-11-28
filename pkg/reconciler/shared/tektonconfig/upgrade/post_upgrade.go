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
	"fmt"
	"slices"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	upgrade "github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/upgrade/helper"
	"go.uber.org/zap"
	apixclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	taskVersiondRetentionCount = 2
)

// performs storage versions upgrade
// lists all the resources and keeps only one storage version
func upgradeStorageVersion(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// resources to be upgraded
	crdGroups := []string{

		// dashboard
		"extensions.dashboard.tekton.dev",

		// pipelines
		"clustertasks.tekton.dev",
		"customruns.tekton.dev",
		"pipelineruns.tekton.dev",
		"pipelines.tekton.dev",
		"taskruns.tekton.dev",
		"tasks.tekton.dev",
		"verificationpolicies.tekton.dev",
		"resolutionrequests.resolution.tekton.dev",

		// Pipelines-as-code
		"repositories.pipelinesascode.tekton.dev",

		// triggers
		"clusterinterceptors.triggers.tekton.dev",
		"clustertriggerbindings.triggers.tekton.dev",
		"eventlisteners.triggers.tekton.dev",
		"interceptors.triggers.tekton.dev",
		"triggerbindings.triggers.tekton.dev",
		"triggers.triggers.tekton.dev",
		"triggertemplates.triggers.tekton.dev",
	}

	migrator := upgrade.NewMigrator(
		dynamic.NewForConfigOrDie(restConfig),
		apixclient.NewForConfigOrDie(restConfig),
		logger,
	)

	upgrade.MigrateStorageVersion(ctx, logger, migrator, crdGroups)

	return nil
}

// removeClusterTaskInstallerSets removes clusterTask, community clusterTask and all versioned clusterTask from the cluster
// as clusterTask has been removed
func removeClusterTaskInstallerSets(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {

	if !v1alpha1.IsOpenShiftPlatform() {
		return nil
	}

	clusterInstallerSetsList := []string{"ClusterTask", "CommunityClusterTask", "VersionedClusterTask"}
	tisClient := operatorClient.OperatorV1alpha1().TektonInstallerSets()

	for _, clusterIS := range clusterInstallerSetsList {
		installerSetsLabelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1alpha1.InstallerSetType: fmt.Sprintf("%s-%s", "custom", strings.ToLower(clusterIS)),
			},
		}
		installerSetsLabel, err := common.LabelSelector(installerSetsLabelSelector)
		if err != nil {
			return err
		}
		// deletes clusterTask installersets
		if err := tisClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: installerSetsLabel,
		}); err != nil {
			logger.Errorw("failed to delete a installerset", "installerSetName", clusterIS, err)
			return err
		}
	}
	return nil
}

// removeVersionedTaskInstallerSets removes the versioned resolver tasks installersets except latest 2 versions
func removeVersionedTaskInstallerSets(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {

	if !v1alpha1.IsOpenShiftPlatform() {
		return nil
	}

	taskInstallerSetsLabelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: fmt.Sprintf("%s-%s", "custom", "versionedresolvertask"),
		},
	}
	taskInstallerSetsLabel, err := common.LabelSelector(taskInstallerSetsLabelSelector)
	if err != nil {
		return err
	}

	return findAndDeleteInstallerSetsByLabelName(ctx, logger, operatorClient, taskInstallerSetsLabel)
}

// removeVersionedStepActionsInstallerSets removes the versioned resolver step actions installersets except latest 2 versions
func removeVersionedStepActionsInstallerSets(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {

	if !v1alpha1.IsOpenShiftPlatform() {
		return nil
	}

	stepActionsInstallerSetsLabelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: fmt.Sprintf("%s-%s", "custom", "versionedresolverstepaction"),
		},
	}
	stepActionsInstallerSetsLabel, err := common.LabelSelector(stepActionsInstallerSetsLabelSelector)
	if err != nil {
		return err
	}

	return findAndDeleteInstallerSetsByLabelName(ctx, logger, operatorClient, stepActionsInstallerSetsLabel)
}

func findAndDeleteInstallerSetsByLabelName(ctx context.Context, logger *zap.SugaredLogger, operatorClient versioned.Interface, installerSetsLabel string) error {
	tsClient := operatorClient.OperatorV1alpha1().TektonInstallerSets()

	installerSets, err := tsClient.List(ctx, metav1.ListOptions{LabelSelector: installerSetsLabel})
	if err != nil {
		return err
	}
	if len(installerSets.Items) < taskVersiondRetentionCount {
		return nil
	}

	installerListName := []string{}
	for _, taskIS := range installerSets.Items {
		installerListName = append(installerListName, taskIS.Name)
	}

	slices.Sort(installerListName)
	slices.Reverse(installerListName)

	for i := taskVersiondRetentionCount; i < len(installerListName); i++ {
		if err := tsClient.Delete(ctx, installerListName[i], metav1.DeleteOptions{}); err != nil {
			logger.Errorw("failed to delete a installerset", "installerSetName", installerListName[i], err)
			return err
		}
	}
	return nil
}
