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

package openshiftpipelinesascode

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	pacSettings "github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	pipelinesAsCodeCM                 = "pipelines-as-code"
	additionalPACControllerNameSuffix = "-pac-controller"
	deprecatedHubCatalogName          = "hub-catalog-name"
	deprecatedHubURL                  = "hub-url"
	tektonHubURL                      = "https://api.hub.tekton.dev/v1"
	tektonHubCatalogName              = "tekton"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		pac := comp.(*v1alpha1.OpenShiftPipelinesAsCode)
		// installerSet adds it's owner as namespace's owner
		// so deleting tekton addon deletes target namespace too
		// to skip it we filter out namespace
		pacManifest := manifest.Filter(mf.Not(mf.ByKind("Namespace")))

		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PacImagePrefix))
		// Run transformers
		tfs := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(openshift.OperandOpenShiftPipelineAsCode),
			common.DeploymentImages(images),
			common.DeploymentEnvVarKubernetesMinVersion(),
			common.AddConfiguration(pac.Spec.Config),
			occommon.ApplyCABundlesToDeployment,
			common.CopyConfigMap(pipelinesAsCodeCM, pac.Spec.Settings),
			removeDeprecatedPACSettings(), // Remove deprecated hub settings to prevent reconciliation loops
			occommon.UpdateServiceMonitorTargetNamespace(pac.Spec.TargetNamespace),
		}

		allTfs := append(tfs, extension.Transformers(pac)...)
		if err := common.Transform(ctx, &pacManifest, pac, allTfs...); err != nil {
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, &pacManifest, pac.Spec.GetTargetNamespace(), pac.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return &pacManifest, nil
	}
}

func additionalControllerTransform(extension common.Extension, name string) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		pac := comp.(*v1alpha1.OpenShiftPipelinesAsCode)
		additionalPACControllerConfig := pac.Spec.PACSettings.AdditionalPACControllers[name]

		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PacImagePrefix))
		// Run transformers
		tfs := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(openshift.OperandOpenShiftPipelineAsCode),
			common.DeploymentImages(images),
			common.AddConfiguration(pac.Spec.Config),
			occommon.ApplyCABundlesToDeployment,
			occommon.UpdateServiceMonitorTargetNamespace(pac.Spec.TargetNamespace),
			updateAdditionControllerDeployment(additionalPACControllerConfig, name),
			updateAdditionControllerService(name),
			updateAdditionControllerConfigMap(additionalPACControllerConfig),
			removeDeprecatedPACSettings(), // Remove deprecated hub settings to prevent reconciliation loops
			updateAdditionControllerRoute(name),
			updateAdditionControllerServiceMonitor(name),
		}

		allTfs := append(tfs, extension.Transformers(pac)...)
		if err := common.Transform(ctx, manifest, pac, allTfs...); err != nil {
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, pac.Spec.GetTargetNamespace(), pac.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return manifest, nil
	}
}

// This returns all resources to deploy for the additional PACController
func filterAdditionalControllerManifest(manifest mf.Manifest) mf.Manifest {
	// filter deployment
	deploymentManifest := manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Deployment")))

	// filter service
	serviceManifest := manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Service")))

	// filter route
	routeManifest := manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller"), mf.ByKind("Route")))

	// filter configmap
	cmManifest := manifest.Filter(mf.All(mf.ByName("pipelines-as-code"), mf.ByKind("ConfigMap")))

	// filter serviceMonitor
	serviceMonitorManifest := manifest.Filter(mf.All(mf.ByName("pipelines-as-code-controller-monitor"), mf.ByKind("ServiceMonitor")))

	filteredManifest := mf.Manifest{}
	filteredManifest = filteredManifest.Append(cmManifest, deploymentManifest, serviceManifest, serviceMonitorManifest, routeManifest)
	return filteredManifest
}

// This updates additional PACController deployment
func updateAdditionControllerDeployment(config v1alpha1.AdditionalPACControllerConfig, name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		u.SetName(fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix))

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		d.Spec.Selector.MatchLabels["app.kubernetes.io/name"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)

		d.Spec.Template.Labels["app"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		d.Spec.Template.Labels["app.kubernetes.io/name"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)

		for i, container := range d.Spec.Template.Spec.Containers {
			container.Name = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
			containerEnvs := d.Spec.Template.Spec.Containers[i].Env
			d.Spec.Template.Spec.Containers[i].Env = replaceEnvInDeployment(containerEnvs, config, name)
			d.Spec.Template.Spec.Containers[i] = container
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// This updates additional PACController Service
func updateAdditionControllerService(name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Service" {
			return nil
		}
		u.SetName(fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix))

		service := &corev1.Service{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, service)
		if err != nil {
			return err
		}

		labels := service.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		labels["app"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		labels["app.kubernetes.io/name"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		service.SetLabels(labels)

		labelSelector := service.Spec.Selector
		if labelSelector == nil {
			labelSelector = map[string]string{}
		}
		labelSelector["app.kubernetes.io/name"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		service.Spec.Selector = labelSelector

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(service)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// This updates additional PACController configMap and sets settings data to configMap data
func updateAdditionControllerConfigMap(config v1alpha1.AdditionalPACControllerConfig) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// set the name
		// set the namespace
		// set the data from settings
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		u.SetName(config.ConfigMapName)

		// apply the defaults here, we are not adding the defaults in CR
		if config.Settings == nil {
			config.Settings = map[string]string{}
		}

		defaultPacSettings := pacSettings.DefaultSettings()
		err := pacSettings.SyncConfig(zap.NewNop().Sugar(), &defaultPacSettings, config.Settings, pacSettings.DefaultValidators())
		if err != nil {
			return err
		}
		config.Settings = v1alpha1.ConvertPacStructToConfigMap(&defaultPacSettings)

		cm := &corev1.ConfigMap{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		for key, value := range config.Settings {
			cm.Data[key] = value
		}
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil

	}
}

// This updates additional PACController route
func updateAdditionControllerRoute(name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Route" {
			return nil
		}
		u.SetName(fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix))

		route := &routev1.Route{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, route)
		if err != nil {
			return err
		}

		route.Spec.To.Name = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		labels := route.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		labels["app"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		labels["pipelines-as-code/route"] = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		route.SetLabels(labels)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(route)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// This updates additional PACController ServiceMonitor
func updateAdditionControllerServiceMonitor(name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ServiceMonitor" {
			return nil
		}

		u.SetName(fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix))
		err := unstructured.SetNestedMap(u.Object, map[string]interface{}{
			"app": fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix),
		}, "spec", "selector", "matchLabels")
		if err != nil {
			return err
		}
		return nil
	}
}

// removeDeprecatedPACSettings removes deprecated hub-catalog-name and hub-url settings
// from the pipelines-as-code ConfigMap to prevent reconciliation loops.
func removeDeprecatedPACSettings() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}
		if u.GetName() != pipelinesAsCodeCM {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		if cm.Data == nil {
			return nil
		}

		modified := false
		// Remove hub-catalog-name if it's set to the deprecated "tekton" value
		if catalogName, exists := cm.Data[deprecatedHubCatalogName]; exists && catalogName == tektonHubCatalogName {
			delete(cm.Data, deprecatedHubCatalogName)
			modified = true
		}

		// Remove hub-url if it's set to the deprecated Tekton Hub API URL
		if hubURL, exists := cm.Data[deprecatedHubURL]; exists && hubURL == tektonHubURL {
			delete(cm.Data, deprecatedHubURL)
			modified = true
		}

		// Only update the unstructured object if we made changes
		if modified {
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
			if err != nil {
				return err
			}
			u.SetUnstructuredContent(unstrObj)
		}

		return nil
	}
}

// This replaces additional PACController deployment's container env
func replaceEnvInDeployment(envs []corev1.EnvVar, envInfo v1alpha1.AdditionalPACControllerConfig, name string) []corev1.EnvVar {
	for i, e := range envs {
		if e.Name == "PAC_CONTROLLER_CONFIGMAP" {
			envs[i].Value = envInfo.ConfigMapName
		}
		if e.Name == "PAC_CONTROLLER_SECRET" {
			envs[i].Value = envInfo.SecretName
		}
		if e.Name == "PAC_CONTROLLER_LABEL" {
			envs[i].Value = fmt.Sprintf("%s%s", name, additionalPACControllerNameSuffix)
		}
	}
	return envs
}
