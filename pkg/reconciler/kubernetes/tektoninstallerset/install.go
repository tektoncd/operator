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
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

const (
	replicasForHash = 999
)

var (
	namespacePred                      = mf.ByKind("Namespace")
	configMapPred                      = mf.ByKind("ConfigMap")
	secretPred                         = mf.ByKind("Secret")
	deploymentPred                     = mf.ByKind("Deployment")
	servicePred                        = mf.ByKind("Service")
	serviceAccountPred                 = mf.ByKind("ServiceAccount")
	cronJobPred                        = mf.ByKind("CronJob")
	eventListenerPred                  = mf.ByKind("EventListener")
	triggerBindingPred                 = mf.ByKind("TriggerBinding")
	triggerTemplatePred                = mf.ByKind("TriggerTemplate")
	rolePred                           = mf.ByKind("Role")
	roleBindingPred                    = mf.ByKind("RoleBinding")
	clusterRolePred                    = mf.ByKind("ClusterRole")
	clusterRoleBindingPred             = mf.ByKind("ClusterRoleBinding")
	validatingWebhookConfigurationPred = mf.ByKind("ValidatingWebhookConfiguration")
	mutatingWebhookConfigurationPred   = mf.ByKind("MutatingWebhookConfiguration")
	horizontalPodAutoscalerPred        = mf.ByKind("HorizontalPodAutoscaler")
	clusterInterceptorPred             = mf.ByKind("ClusterInterceptor")
	clusterTaskPred                    = mf.ByKind("ClusterTask")
	clusterTriggerBindingPred          = mf.ByKind("ClusterTriggerBinding")
	pipelinePred                       = mf.ByKind("Pipeline")

	// OpenShift Specific
	securityContextConstraints = mf.ByKind("SecurityContextConstraints")
	serviceMonitorPred         = mf.ByKind("ServiceMonitor")
	routePred                  = mf.ByKind("Route")
	consoleCLIDownloadPred     = mf.ByKind("ConsoleCLIDownload")
	consoleQuickStartPred      = mf.ByKind("ConsoleQuickStart")
	ConsoleYAMLSamplePred      = mf.ByKind("ConsoleYAMLSample")
)

type installer struct {
	Manifest mf.Manifest
}

func ensureResources(mani *mf.Manifest) error {
	freshCreate := true
	// check if all resources in a given set of resources exists
	ok, err := allResourcesExists(mani)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	// if error == NotFound error or !ok
	// then Apply all resources
	// if ok, that means the reosource already exists
	// but this reconcile could be an modification eg: concig-defaults change
	// set freshCreate flag to false, and then Apply all
	// so that we can skip (break) RECONCILE_AGAIN loop path
	if ok {
		freshCreate = false
	}

	if err := mani.Apply(); err != nil {
		return err
	}
	// on err == nil after Apply() return RECONCILE_AGAIN
	// if freshCreate == true return RECONCILE_AGAIN
	// this ensures freshly created resources are in place before proceeding to next stages of reconciler
	if freshCreate {
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}

func (i *installer) EnsureCRDs() error {
	resourceList := i.Manifest.Filter(mf.Any(mf.CRDs))
	return ensureResources(&resourceList)
}

func (i *installer) EnsureClusterScopedResources() error {
	resourceList := i.Manifest.Filter(
		mf.Any(
			namespacePred,
			clusterRolePred,
			validatingWebhookConfigurationPred,
			mutatingWebhookConfigurationPred,
			clusterInterceptorPred,
			clusterTaskPred,
			clusterTriggerBindingPred,
			consoleCLIDownloadPred,
			consoleQuickStartPred,
			ConsoleYAMLSamplePred,
			securityContextConstraints,
		))
	return ensureResources(&resourceList)
}

func (i *installer) EnsureNamespaceScopedResources() error {
	resourceList := i.Manifest.Filter(
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
			cronJobPred,
			eventListenerPred,
			triggerBindingPred,
			triggerTemplatePred,
			servicePred,
			routePred,
		))
	return ensureResources(&resourceList)
}

func (i *installer) EnsureDeploymentResources() error {

	for _, d := range i.Manifest.Filter(mf.Any(deploymentPred)).Resources() {
		if err := i.ensureDeployment(&d); err != nil {
			return err
		}
	}

	return nil
}

func computeDeploymentHash(d appsv1.Deployment) (string, error) {
	// set replicas to a constant value and then calculate hash so
	// that later if user updates replicas, we can exclude that change.
	// setting the replicas to same const and checking the hash
	// so that we can allow only replica change revert any other change
	// done to the deployment spec
	d.Spec.Replicas = ptr.Int32(replicasForHash)

	return hash.Compute(d.Spec)
}

func (i *installer) createDeployment(expected *unstructured.Unstructured) error {

	dep := &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(expected.Object, dep)
	if err != nil {
		return err
	}

	hash, err := computeDeploymentHash(*dep)
	if err != nil {
		return fmt.Errorf("failed to compute hash of deployment: %v", err)
	}

	if len(dep.Annotations) == 0 {
		dep.Annotations = map[string]string{}
	}
	dep.Annotations[v1alpha1.LastAppliedHashKey] = hash

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	expected.SetUnstructuredContent(unstrObj)

	return i.Manifest.Client.Create(expected)
}

func (i *installer) updateDeployment(existing *unstructured.Unstructured, existingDeployment, expectedDeployment *appsv1.Deployment) error {

	// save on cluster replicas in a var and assign it back to deployment
	onClusterReplicas := existingDeployment.Spec.Replicas

	existingDeployment.Spec = expectedDeployment.Spec
	existingDeployment.Spec.Replicas = onClusterReplicas

	// compute new hash of spec and add as annotation
	newHash, err := computeDeploymentHash(*existingDeployment)
	if err != nil {
		return fmt.Errorf("failed to compute new hash of existing deployment: %v", err)
	}

	if len(existingDeployment.Annotations) == 0 {
		existingDeployment.Annotations = map[string]string{}
	}

	existingDeployment.Annotations[v1alpha1.LastAppliedHashKey] = newHash

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existingDeployment)
	if err != nil {
		return err
	}
	existing.SetUnstructuredContent(unstrObj)

	err = i.Manifest.Client.Update(existing)
	if err != nil {
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return err
}

func (i *installer) ensureDeployment(expected *unstructured.Unstructured) error {

	// check if deployment already exist
	existing, err := i.Manifest.Client.Get(expected)
	if err != nil {

		// If deployment doesn't exist, then create new
		if apierrs.IsNotFound(err) {
			errInner := i.createDeployment(expected)
			if errInner == nil {
				return v1alpha1.RECONCILE_AGAIN_ERR
			}
			return errInner
		}
		return err
	}

	// if already exist then check if spec is changed
	existingDeployment := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(existing.Object, existingDeployment); err != nil {
		return err
	}

	expectedDeployment := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(expected.Object, expectedDeployment); err != nil {
		return err
	}

	// compare existing deployment spec hash with the one saved in annotation
	// if annotation doesn't exist then update the deployment

	existingDepSpecHash, err := computeDeploymentHash(*existingDeployment)
	if err != nil {
		return fmt.Errorf("failed to compute hash of existing deployment: %v", err)
	}

	hashFromAnnotation, hashExist := existingDeployment.Annotations[v1alpha1.LastAppliedHashKey]

	// if hash doesn't exist then update the deployment with hash
	if !hashExist {
		return i.updateDeployment(existing, existingDeployment, expectedDeployment)
	}

	// if both hashes are same, that means deployment on cluster is the same as when it
	// was created (there may be change in replica which we allow)
	if existingDepSpecHash == hashFromAnnotation {

		// there might be a case where deployment in installerSet spec might have changed
		// compare the expected deployment spec hash with the hash in annotation
		expectedDepSpecHash, err := computeDeploymentHash(*expectedDeployment)
		if err != nil {
			return fmt.Errorf("failed to compute hash of expected deployment: %v", err)
		}

		if expectedDepSpecHash != hashFromAnnotation {
			return i.updateDeployment(existing, existingDeployment, expectedDeployment)
		}

		return nil
	}

	// hash is changed so revert back to original deployment
	// keeping the replicas change if exist

	return i.updateDeployment(existing, existingDeployment, expectedDeployment)
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

func allResourcesExists(m *mf.Manifest) (bool, error) {
	c := m.Client
	for _, item := range m.Resources() {
		ok, err := resourceExists(c, &item)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func resourceExists(c mf.Client, u *unstructured.Unstructured) (bool, error) {
	_, err := c.Get(u)
	if err != nil {
		return false, err
	}
	return true, nil
}
