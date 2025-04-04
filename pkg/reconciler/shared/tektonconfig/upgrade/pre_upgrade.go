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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
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

// previous version of the TektonConfig CR's addon params has cluster task params to manage the cluster tasks
// and cluster tasks have been deprecated and removed so need to remove the clusterTasks and communityClusterTasks
// params from TektonConfig's addon params and this removes the cluster tasks params and updates TektonConfig's addon params
// Todo: remove this in the next operator release
func removeDeprecatedAddonParams(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	tcCR, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	updatedParams := []v1alpha1.Param{}
	for _, p := range tcCR.Spec.Addon.Params {
		if p.Name == "clusterTasks" || p.Name == "communityClusterTasks" {
			continue
		}
		updatedParams = append(updatedParams, p)
	}

	// update the Tekton config's addon params
	tcCR.Spec.Addon.Params = updatedParams
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tcCR, metav1.UpdateOptions{})
	return err
}

// previous version of the Tekton Operator does not install the TektonResult via TektonConfig
// in the new version, we are supporting to installing and manage TektonResult via TektonConfig
// so if TektonResult CR exists on the cluster then needs to copy the TektonResult CR configuration to the TektonConfig CR
func copyResultConfigToTektonConfig(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// get the TekonResult CR
	trCR, err := operatorClient.OperatorV1alpha1().TektonResults().Get(ctx, v1alpha1.ResultResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// get the TekonConfig CR
	tcCR, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// copy the existing TektonResult CR configuration  to the TektonConfig CR
	tcCR.Spec.Result.ResultsAPIProperties = trCR.Spec.ResultsAPIProperties
	tcCR.Spec.Result.LokiStackProperties = trCR.Spec.LokiStackProperties
	tcCR.Spec.Result.Options = trCR.Spec.Options

	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tcCR, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
