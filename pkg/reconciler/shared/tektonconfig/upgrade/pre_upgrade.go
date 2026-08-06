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
	"encoding/json"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpruner"
	"gopkg.in/yaml.v3"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonresult "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonresult"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

const (
	deprecatedTektonSchedulerFinalizer          = "tektonschedulers.operator.tekton.dev"
	deprecatedTektonSchedulerClusterRole        = "tekton-scheduler-role"
	deprecatedTektonSchedulerClusterRoleBinding = "tekton-scheduler-rolebinding"
)

// previous version of tekton operator uses a condition type called "InstallSucceeded" in status
// but in the recent version we do not have that field, hence "InstallSucceeded" condition never updated.
// for some reason, if it was in failed state, tektonConfig CR always in failed state
// even though all the resources are up and running. as the operator sums all the status conditions
// to avoid this, remove all the existing conditions from the status of the CR.
// conditions will be repopulated
func resetTektonConfigConditions(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// fetch the current tektonConfig CR
	tcCR, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// remove all the existing conditions
	tcCR.Status.Conditions = duckv1.Conditions{}
	// update the status
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().UpdateStatus(ctx, tcCR, metav1.UpdateOptions{})
	return err
}

// migrateTektonSchedulerToTektonKueue preserves the deprecated API long enough
// to migrate existing configuration. The old CRD remains served, but no longer
// reconciles operand resources.
func migrateTektonSchedulerToTektonKueue(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return err
	}
	if err == nil && tc.Spec.MigrateScheduler() {
		if _, err := operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{}); err != nil {
			return err
		}
		logger.Info("Migrated TektonConfig spec.scheduler to spec.kueue")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	legacySelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{
		v1alpha1.CreatedByKey: v1alpha1.SchedulerCreatedByValue,
	}})
	legacyInstallerSets, err := operatorClient.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{LabelSelector: legacySelector})
	if err != nil {
		return err
	}
	if len(legacyInstallerSets.Items) > 0 {
		for _, installerSet := range legacyInstallerSets.Items {
			if err := operatorClient.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, installerSet.Name, metav1.DeleteOptions{}); err != nil && !apierrs.IsNotFound(err) {
				return err
			}
		}
		logger.Infof("Removed %d deprecated TektonScheduler InstallerSets", len(legacyInstallerSets.Items))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	deletedLegacyRBAC := false
	if k8sClient != nil {
		if err := k8sClient.RbacV1().ClusterRoleBindings().Delete(ctx, deprecatedTektonSchedulerClusterRoleBinding, metav1.DeleteOptions{}); err == nil {
			deletedLegacyRBAC = true
		} else if !apierrs.IsNotFound(err) {
			return err
		}
		if err := k8sClient.RbacV1().ClusterRoles().Delete(ctx, deprecatedTektonSchedulerClusterRole, metav1.DeleteOptions{}); err == nil {
			deletedLegacyRBAC = true
		} else if !apierrs.IsNotFound(err) {
			return err
		}
	}
	if deletedLegacyRBAC {
		logger.Info("Removed deprecated TektonScheduler RBAC")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	legacy, err := operatorClient.OperatorV1alpha1().TektonSchedulers().Get(ctx, v1alpha1.TektonSchedulerResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	if _, err := operatorClient.OperatorV1alpha1().TektonKueues().Get(ctx, v1alpha1.TektonKueueResourceName, metav1.GetOptions{}); apierrs.IsNotFound(err) {
		kueue := &v1alpha1.TektonKueue{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       v1alpha1.KindTektonKueue,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            v1alpha1.TektonKueueResourceName,
				Labels:          legacy.Labels,
				Annotations:     legacy.Annotations,
				OwnerReferences: legacy.OwnerReferences,
			},
			Spec: v1alpha1.TektonKueueSpec{
				CommonSpec:    legacy.Spec.CommonSpec,
				Kueue:         legacy.Spec.Scheduler.ToKueue(),
				NetworkPolicy: legacy.Spec.NetworkPolicy,
			},
		}
		if _, err := operatorClient.OperatorV1alpha1().TektonKueues().Create(ctx, kueue, metav1.CreateOptions{}); err != nil {
			return err
		}
		logger.Info("Migrated TektonScheduler resource to TektonKueue")
		return v1alpha1.REQUEUE_EVENT_AFTER
	} else if err != nil {
		return err
	}

	finalizers := make([]string, 0, len(legacy.Finalizers))
	for _, finalizer := range legacy.Finalizers {
		if finalizer != deprecatedTektonSchedulerFinalizer {
			finalizers = append(finalizers, finalizer)
		}
	}
	if len(finalizers) != len(legacy.Finalizers) {
		patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{"finalizers": finalizers}})
		if err != nil {
			return err
		}
		if _, err := operatorClient.OperatorV1alpha1().TektonSchedulers().Patch(ctx, legacy.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			return err
		}
		logger.Info("Removed the operator finalizer from migrated TektonScheduler resource")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if err := operatorClient.OperatorV1alpha1().TektonSchedulers().Delete(ctx, legacy.Name, metav1.DeleteOptions{}); err != nil && !apierrs.IsNotFound(err) {
		return err
	}
	logger.Info("Requested deletion of migrated TektonScheduler resource")
	return v1alpha1.REQUEUE_EVENT_AFTER
}

// previous version of the tekton operator uses default value which is false for enable-step-actions.
// In the new version, we are shipping stepAction, allowing users to create tasks using stepAction.
// Therefore, we are changing the enable-step-actions setting from false to true.
func upgradePipelineProperties(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// fetch the current tektonConfig CR
	tcCR, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// For historical reasons, if it is upgraded from a historical version, this field may be nil
	if tcCR.Spec.Pipeline.EnableStepActions == nil || !*tcCR.Spec.Pipeline.EnableStepActions {
		// update enable-step-actions to true from false which is default.
		tcCR.Spec.Pipeline.EnableStepActions = ptr.Bool(true)
		_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tcCR, metav1.UpdateOptions{})
		return err
	}
	return nil
}

// previous version of the Tekton Operator created default tekton-results-tls on Openshift Platform
// causing it Tekton Results api failure
func deleteTektonResultsTLSSecret(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	if !v1alpha1.IsOpenShiftPlatform() {
		return nil
	}

	// get the TekonResult CR
	trCR, err := operatorClient.OperatorV1alpha1().TektonResults().Get(ctx, v1alpha1.ResultResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// get the tekton-results-tls secret
	tlsSecret, err := k8sClient.CoreV1().Secrets(trCR.Spec.TargetNamespace).Get(ctx, tektonresult.TlsSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// delete default tekton-results-tls secret which has no OwnerReferences
	if len(tlsSecret.OwnerReferences) == 0 {
		err = k8sClient.CoreV1().Secrets(trCR.Spec.TargetNamespace).Delete(ctx, tektonresult.TlsSecretName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: Remove the preUpgradeTektonPruner upgrade function in next operator release
func preUpgradeTektonPruner(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// get tektonConfig CR
	logger.Infof("Performing Preupgrade for TektonPruner")
	tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		logger.Errorw("error on getting TektonConfig CR", err)
		return err
	}

	if tc.Spec.TektonPruner.IsDisabled() {
		logger.Infof("TektonPruner is disabled, skipping pre-upgrade for TektonPruner")
		return nil
	}

	var prunerConfig v1alpha1.TektonPrunerConfig
	cm, err := k8sClient.CoreV1().ConfigMaps(tc.Spec.TargetNamespace).Get(ctx, tektonpruner.PrunerConfigMapName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			prunerConfig = v1alpha1.TektonPrunerConfig{}
		}
	}
	key := "global-config"
	if cm != nil && cm.Data[key] != "" {
		if err := yaml.Unmarshal([]byte(cm.Data[key]), &prunerConfig.GlobalConfig); err != nil {
			logger.Errorf("error on Unmarshal TektonPruner ConfigMap data", err)
			return err
		}
	}

	tc.Spec.TektonPruner.GlobalConfig = prunerConfig.GlobalConfig

	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{})
	return err
}

// preUpgradePipelinesAsCodeArtifacts checks if Pipelines as Code is installed and updates
// the hub catalog settings to use the artifact hub URL. It cleans up hub-catalog-name from:
// 1. TektonConfig CR settings
// 2. OpenShiftPipelinesAsCode CR settings
// 3. pipelines-as-code config map
func preUpgradePipelinesAsCodeArtifacts(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// Only run on OpenShift platform
	if !v1alpha1.IsOpenShiftPlatform() {
		logger.Infof("Not on OpenShift platform, skipping Pipelines as Code artifact upgrade")
		return nil
	}

	// Get TektonConfig CR
	logger.Infof("Performing preupgrade for Pipelines as Code artifact settings")
	tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			logger.Infof("TektonConfig CR not found, skipping Pipelines as Code artifact upgrade")
			return nil
		}
		logger.Errorw("error on getting TektonConfig CR", err)
		return err
	}

	pacSpec := tc.Spec.Platforms.OpenShift.PipelinesAsCode
	// Check if Pipelines as Code is enabled
	if pacSpec == nil || pacSpec.Enable == nil || !*pacSpec.Enable {
		logger.Infof("Pipelines as Code is not enabled, skipping artifact upgrade")
		return nil
	}

	// Initialize settings if nil
	if pacSpec.PACSettings.Settings == nil {
		pacSpec.PACSettings.Settings = make(map[string]string)
	}

	// Fetch PAC settings
	settings := pacSpec.PACSettings.Settings

	// Set hub-catalog-type to artifacthub if not already set or if it's set to tektonhub
	if catalogType, exists := settings["hub-catalog-type"]; !exists || catalogType == "tektonhub" {
		settings["hub-catalog-type"] = "artifacthub"
		logger.Infof("Updated hub-catalog-type to artifacthub")
	}

	// Set hub-url to https://artifacthub.io if not already set or if it's set to the old API URL
	if hubURL, exists := settings["hub-url"]; !exists || hubURL == "https://artifacthub.io/api/v1" || hubURL == "https://api.hub.tekton.dev/v1" {
		settings["hub-url"] = "https://artifacthub.io"
		logger.Infof("Updated hub-url to https://artifacthub.io")
	}

	// remove hub-catalog-name key from setting if found
	if _, exists := settings["hub-catalog-name"]; exists {
		delete(settings, "hub-catalog-name")
		logger.Infof("Removed hub-catalog-name field from TektonConfig CR")
	}

	// Update the TektonConfig CR
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{})
	if err != nil {
		logger.Errorw("error updating TektonConfig CR with artifact settings", err)
		return err
	}

	// Also check and update the OpenShiftPipelinesAsCode CR if it exists
	err = updateOpenShiftPipelinesAsCodeCR(ctx, logger, operatorClient)
	if err != nil {
		logger.Errorw("error updating OpenShiftPipelinesAsCode CR", err)
		return err
	}

	// Also check and update the deployed pipelines-as-code config map if it exists
	err = updatePipelinesAsCodeConfigMap(ctx, logger, k8sClient, tc.Spec.TargetNamespace)
	if err != nil {
		logger.Errorw("error updating pipelines-as-code config map", err)
		return err
	}

	logger.Infof("Successfully updated Pipelines as Code artifact settings in TektonConfig CR, OpenShiftPipelinesAsCode CR, and config map")
	return nil
}

// updatePipelinesAsCodeConfigMap checks and updates the deployed pipelines-as-code config map
// to remove hub-catalog-name if it exists
func updatePipelinesAsCodeConfigMap(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, targetNamespace string) error {
	configMapName := "pipelines-as-code"

	// First check if the config map exists and has the hub-catalog-name field
	cm, err := k8sClient.CoreV1().ConfigMaps(targetNamespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			logger.Infof("pipelines-as-code config map not found, skipping config map update")
			return nil
		}
		return err
	}

	// Check if hub-catalog-name exists in the config map data
	if cm.Data == nil {
		logger.Infof("pipelines-as-code config map has no data, skipping config map update")
		return nil
	}

	// Check if hub-catalog-name key exists
	if val, exists := cm.Data["hub-catalog-name"]; exists {
		// Create a patch to remove the hub-catalog-name field
		// Setting the field to null in the patch will remove it
		patch := map[string]interface{}{
			"data": map[string]interface{}{
				"hub-catalog-name": nil,
			},
		}

		patchBytes, err := json.Marshal(patch)
		if err != nil {
			logger.Errorf("failed to marshal patch payload: %v", err)
			return err
		}

		// Apply the patch to remove the hub-catalog-name field
		_, err = k8sClient.CoreV1().ConfigMaps(targetNamespace).Patch(ctx, configMapName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			logger.Errorf("failed to patch pipelines-as-code config map: %v", err)
			return err
		}

		if val == "" {
			logger.Infof("Removed empty hub-catalog-name field from pipelines-as-code config map")
		} else {
			logger.Infof("Removed hub-catalog-name field (value: %s) from pipelines-as-code config map", val)
		}
		logger.Infof("Successfully updated pipelines-as-code config map")
	} else {
		logger.Infof("No catalog name entries found in pipelines-as-code config map, no update needed")
	}

	return nil
}

// updateOpenShiftPipelinesAsCodeCR checks and updates the OpenShiftPipelinesAsCode CR
// to remove hub-catalog-name if it exists
func updateOpenShiftPipelinesAsCodeCR(ctx context.Context, logger *zap.SugaredLogger, operatorClient versioned.Interface) error {
	// First check if the OpenShiftPipelinesAsCode CR exists and has the hub-catalog-name field
	pacCR, err := operatorClient.OperatorV1alpha1().OpenShiftPipelinesAsCodes().Get(ctx, v1alpha1.OpenShiftPipelinesAsCodeName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			logger.Infof("OpenShiftPipelinesAsCode CR not found, skipping PAC CR update")
			return nil
		}
		return err
	}

	// Check if PAC settings exist
	if pacCR.Spec.PACSettings.Settings == nil {
		logger.Infof("OpenShiftPipelinesAsCode CR has no settings, skipping PAC CR update")
		return nil
	}

	// Check if hub-catalog-name key exists
	if val, exists := pacCR.Spec.PACSettings.Settings["hub-catalog-name"]; exists {
		// Create a patch to remove the hub-catalog-name field
		// Setting the field to null in the patch will remove it
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"settings": map[string]interface{}{
					"hub-catalog-name": nil,
				},
			},
		}

		patchBytes, err := json.Marshal(patch)
		if err != nil {
			logger.Errorf("failed to marshal patch payload: %v", err)
			return err
		}

		// Apply the patch to remove the hub-catalog-name field
		_, err = operatorClient.OperatorV1alpha1().OpenShiftPipelinesAsCodes().Patch(ctx, v1alpha1.OpenShiftPipelinesAsCodeName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			logger.Errorf("failed to patch OpenShiftPipelinesAsCode CR: %v", err)
			return err
		}

		if val == "" {
			logger.Infof("Removed empty hub-catalog-name field from OpenShiftPipelinesAsCode CR")
		} else {
			logger.Infof("Removed hub-catalog-name field (value: %s) from OpenShiftPipelinesAsCode CR", val)
		}
		logger.Infof("Successfully updated OpenShiftPipelinesAsCode CR")
	} else {
		logger.Infof("No catalog name entries found in OpenShiftPipelinesAsCode CR, no update needed")
	}

	return nil
}

// removeDeprecatedDisableAffinityAssistant removes the deprecated DisableAffinityAssistant field from tektonConfig CR spec during pre upgrade
// TODO: Remove this upgrade function in the release-v0.80.x
func removeDeprecatedDisableAffinityAssistant(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// use merge patch to remove the deprecated field from the stored CR
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"pipeline": map[string]interface{}{
				"disable-affinity-assistant": nil,
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		logger.Errorf("failed to marshal patch payload: %v", err)
		return err
	}

	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Patch(ctx, v1alpha1.ConfigResourceName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		logger.Debugw("patch to remove disable-affinity-assistant field returned error (field may not exist)", "error", err)
		return nil
	}

	logger.Info("Successfully removed deprecated disable-affinity-assistant field from TektonConfig CR")
	return nil
}

// removeHubFromTektonConfig removes the deprecated hub field from the TektonConfig spec.
// TODO: Remove this function in the release-v0.80.x
func removeHubFromTektonConfig(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	tcCR, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Clear the hub spec if it was previously set
	if len(tcCR.Spec.Hub.Params) > 0 || tcCR.Spec.Hub.Options.Deployments != nil {
		tcCR.Spec.Hub = v1alpha1.Hub{}
		_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tcCR, metav1.UpdateOptions{})
		return err
	}
	return nil
}
