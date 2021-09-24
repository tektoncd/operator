/*
Copyright 2021 The Tekton Authors

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

package tektoninstallerset

import (
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	namespacePred                      = mf.ByKind("Namespace")
	configMapPred                      = mf.ByKind("ConfigMap")
	secretPred                         = mf.ByKind("Secret")
	deploymentPred                     = mf.ByKind("Deployment")
	servicePred                        = mf.ByKind("Service")
	serviceAccountPred                 = mf.ByKind("ServiceAccount")
	rolePred                           = mf.ByKind("Role")
	roleBindingPred                    = mf.ByKind("RoleBinding")
	clusterRolePred                    = mf.ByKind("ClusterRole")
	clusterRoleBindingPred             = mf.ByKind("ClusterRoleBinding")
	podSecurityPolicyPred              = mf.ByKind("PodSecurityPolicy")
	validatingWebhookConfigurationPred = mf.ByKind("ValidatingWebhookConfiguration")
	mutatingWebhookConfigurationPred   = mf.ByKind("MutatingWebhookConfiguration")
	horizontalPodAutoscalerPred        = mf.ByKind("HorizontalPodAutoscaler")
	clusterInterceptorPred             = mf.ByKind("ClusterInterceptor")
	clusterTaskPred                    = mf.ByKind("ClusterTask")
	clusterTriggerBindingPred          = mf.ByKind("ClusterTriggerBinding")
	pipelinePred                       = mf.ByKind("Pipeline")

	// OpenShift Specific
	serviceMonitorPred     = mf.ByKind("ServiceMonitor")
	consoleCLIDownloadPred = mf.ByKind("ConsoleCLIDownload")
	consoleQuickStartPred  = mf.ByKind("ConsoleQuickStart")
	ConsoleYAMLSamplePred  = mf.ByKind("ConsoleYAMLSample")
)

type installer struct {
	Manifest mf.Manifest
}

func (i *installer) EnsureCRDs() error {
	if err := i.Manifest.Filter(mf.Any(mf.CRDs)).Apply(); err != nil {
		return err
	}
	return nil
}

func (i *installer) EnsureClusterScopedResources() error {
	if err := i.Manifest.Filter(
		mf.Any(
			namespacePred,
			clusterRolePred,
			podSecurityPolicyPred,
			validatingWebhookConfigurationPred,
			mutatingWebhookConfigurationPred,
			clusterInterceptorPred,
			clusterTaskPred,
			clusterTriggerBindingPred,
			consoleCLIDownloadPred,
			consoleQuickStartPred,
			ConsoleYAMLSamplePred,
		)).Apply(); err != nil {
		return err
	}
	return nil
}

func (i *installer) EnsureNamespaceScopedResources() error {
	if err := i.Manifest.Filter(
		mf.Any(
			serviceAccountPred,
			clusterRoleBindingPred,
			rolePred,
			roleBindingPred,
			configMapPred,
			secretPred,
			horizontalPodAutoscalerPred,
			pipelinePred,
			serviceMonitorPred,
		)).Apply(); err != nil {
		return err
	}
	return nil
}

func (i *installer) EnsureDeploymentResources() error {
	if err := i.Manifest.Filter(
		mf.Any(
			deploymentPred,
			servicePred,
		)).Apply(); err != nil {
		return err
	}
	return nil
}

func (i *installer) IsWebhookReady() error {

	for _, u := range i.Manifest.Filter(deploymentPred).Resources() {

		if !strings.Contains(u.GetName(), "webhook") {
			continue
		}

		err := i.isDeploymentReady(&u)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *installer) IsControllerReady() error {

	for _, u := range i.Manifest.Filter(deploymentPred).Resources() {

		if !strings.Contains(u.GetName(), "controller") {
			continue
		}

		err := i.isDeploymentReady(&u)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *installer) AllDeploymentsReady() error {

	for _, u := range i.Manifest.Filter(deploymentPred).Resources() {

		if strings.Contains(u.GetName(), "controller") ||
			strings.Contains(u.GetName(), "webhook") {
			continue
		}

		err := i.isDeploymentReady(&u)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *installer) isDeploymentReady(d *unstructured.Unstructured) error {

	resource, err := i.Manifest.Client.Get(d)
	if err != nil {
		return err
	}

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deployment)
	if err != nil {
		return err
	}

	if !isDeploymentAvailable(deployment) {
		return fmt.Errorf("%s deployment not ready", deployment.GetName())
	}

	return nil
}

func isDeploymentAvailable(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
