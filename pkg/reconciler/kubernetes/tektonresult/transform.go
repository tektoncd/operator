/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" B]>SIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tektonresult

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

const (
	// Results ConfigMap
	configAPI                         = "tekton-results-api-config"
	deploymentAPI                     = "tekton-results-api"
	configINFO                        = "tekton-results-info"
	configMetrics                     = "tekton-results-config-observability"
	configPostgresDB                  = "tekton-results-postgres"
	pvcLoggingVolume                  = "tekton-logs"
	apiContainerName                  = "api"
	retentionPolicyAgentContainerName = "retention-policy-agent"
	watcherContainerName              = "watcher"
	googleAPPCredsEnvName             = "GOOGLE_APPLICATION_CREDENTIALS"
	googleCredsVolName                = "google-creds"
	googleCredsPath                   = "/creds/google"

	loggingProxyPath              = "LOGGING_PLUGIN_PROXY_PATH"
	loggingAPIURL                 = "LOGGING_PLUGIN_API_URL"
	loggingTokenPath              = "LOGGING_PLUGIN_TOKEN_PATH"
	loggingNamespaceKey           = "LOGGING_PLUGIN_NAMESPACE_KEY"
	loggingStaticLabels           = "LOGGING_PLUGIN_STATIC_LABELS"
	loggingForwarderDelayDuration = "LOGGING_PLUGIN_FORWARDER_DELAY_DURATION"
	logsAPIKey                    = "LOGS_API"
	logsTypeKey                   = "LOGS_TYPE"

	resultAPIDeployment                          = "tekton-results-api"
	resultWatcherDeployment                      = "tekton-results-watcher"
	resultWatcherContainer                       = "watcher"
	tektonResultWatcherName                      = "tekton-results-watcher"
	tektonResultWatcherServiceName               = "tekton-results-watcher"
	tektonResultWatcherStatefulServiceName       = "STATEFUL_SERVICE_NAME"
	tektonResultWatcherStatefulControllerOrdinal = "STATEFUL_CONTROLLER_ORDINAL"
	tektonResultleaderElectionConfig             = "tekton-results-config-leader-election"
)

var (
	resultDeployementNames = []string{resultAPIDeployment, resultWatcherDeployment}
	// allowed property secret keys
	allowedPropertySecretKeys = []string{
		"S3_BUCKET_NAME",
		"S3_ENDPOINT",
		"S3_HOSTNAME_IMMUTABLE",
		"S3_REGION",
		"S3_ACCESS_KEY_ID",
		"S3_SECRET_ACCESS_KEY",
		"S3_MULTI_PART_SIZE",
	}
	DB_USER     = "DB_USER"
	DB_PASSWORD = "DB_PASSWORD"
	// maps container env key with db secret key
	ContainerEnvKeys = map[string]string{
		DB_USER:     "POSTGRES_USER",
		DB_PASSWORD: "POSTGRES_PASSWORD",
	}
)

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonResult)
	resultImgs := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ResultsImagePrefix))

	targetNs := comp.GetSpec().GetTargetNamespace()
	filterExternalDB(instance, manifest)
	extra := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdResults),
		common.ApplyProxySettings,
		common.ReplaceNamespaceInDeploymentArgs([]string{resultWatcherDeployment}, targetNs),
		common.ReplaceNamespaceInDeploymentEnv(resultDeployementNames, targetNs),
		updateApiConfig(instance.Spec),
		updateApiEnv(instance.Spec),
		enablePVCLogging(instance.Spec.ResultsAPIProperties),
		updateEnvWithSecretName(instance.Spec.ResultsAPIProperties),
		updateEnvWithDBSecretName(instance.Spec.ResultsAPIProperties),
		populateGoogleCreds(instance.Spec.ResultsAPIProperties),
		common.AddDeploymentRestrictedPSA(),
		common.AddConfiguration(instance.Spec.Config),
		common.AddStatefulSetRestrictedPSA(),
		common.DeploymentImages(resultImgs),
		common.DeploymentEnvVarKubernetesMinVersion(),
		common.StatefulSetImages(resultImgs),
		common.AddConfigMapValues(tektonResultleaderElectionConfig, instance.Spec.Performance.PerformanceLeaderElectionConfig),
		common.UpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(&instance.Spec.Performance, tektonResultleaderElectionConfig, resultWatcherDeployment, resultWatcherContainer),
	}

	if instance.Spec.Performance.StatefulsetOrdinals != nil && *instance.Spec.Performance.StatefulsetOrdinals {
		extra = append(extra,
			common.ConvertDeploymentToStatefulSet(tektonResultWatcherName, tektonResultWatcherServiceName),
			common.AddStatefulEnvVars(tektonResultWatcherName, tektonResultWatcherServiceName, tektonResultWatcherStatefulServiceName, tektonResultWatcherStatefulControllerOrdinal),
		)
	}

	extra = append(extra, r.extension.Transformers(instance)...)
	err := common.Transform(ctx, manifest, instance, extra...)
	if err != nil {
		return err
	}

	// additional options transformer
	// always execute as last transformer, so that the values in options will be final update values on the manifests
	if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, instance.Spec.GetTargetNamespace(), instance.Spec.Options); err != nil {
		return err
	}
	return nil
}

func enablePVCLogging(p v1alpha1.ResultsAPIProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if p.LogsAPI == nil || !*p.LogsAPI || p.LoggingPVCName == "" || p.LogsPath == "" || u.GetKind() != "Deployment" || u.GetName() != deploymentAPI {
			return nil
		}

		d := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		updatePVC := true
		for i := 0; i < len(d.Spec.Template.Spec.Containers[0].VolumeMounts); i++ {
			if d.Spec.Template.Spec.Containers[0].VolumeMounts[i].Name == pvcLoggingVolume {
				d.Spec.Template.Spec.Containers[0].VolumeMounts[i] = corev1.VolumeMount{
					Name:      pvcLoggingVolume,
					MountPath: p.LogsPath,
				}
				updatePVC = false
			}
		}
		if updatePVC {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = append(
				d.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{Name: pvcLoggingVolume,
					MountPath: p.LogsPath,
				})
		}

		updatePVC = true
		vol := corev1.Volume{
			Name: pvcLoggingVolume,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: p.LoggingPVCName,
				},
			},
		}
		for i := 0; i < len(d.Spec.Template.Spec.Volumes); i++ {
			if d.Spec.Template.Spec.Volumes[i].Name == pvcLoggingVolume {
				d.Spec.Template.Spec.Volumes[i] = vol
				updatePVC = false
			}
		}
		if updatePVC {
			d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, vol)
		}

		unstrObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func updateApiConfig(s v1alpha1.TektonResultSpec) mf.Transformer {
	p := s.ResultsAPIProperties

	return func(u *unstructured.Unstructured) error {

		kind := strings.ToLower(u.GetKind())
		if kind != "configmap" {
			return nil
		}

		if u.GetName() != configAPI {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		values := reflect.ValueOf(p)
		types := values.Type()
		prop := make(map[string]string)

		applyLokiStackConfig(prop, s.LokiStackProperties)

		if !s.IsExternalDB {
			prop["DB_HOST"] = "tekton-results-postgres-service." + s.TargetNamespace + ".svc.cluster.local"
		}

		for i := 0; i < values.NumField(); i++ {
			key := strings.Split(types.Field(i).Tag.Get("json"), ",")[0]
			if key == "" {
				continue
			}
			ukey := strings.ToUpper(key)

			if values.Field(i).Kind() == reflect.Bool {
				prop[ukey] = strconv.FormatBool(values.Field(i).Bool())
				continue
			}

			if values.Field(i).Kind() == reflect.Int64 {
				prop[ukey] = strconv.FormatInt(values.Field(i).Int(), 10)
				continue
			}

			if values.Field(i).Kind() == reflect.Uint64 {
				prop[ukey] = strconv.FormatUint(values.Field(i).Uint(), 10)
				continue
			}

			if values.Field(i).Kind() == reflect.Ptr {
				innerElem := values.Field(i).Elem()

				if !innerElem.IsValid() {
					continue
				}
				switch innerElem.Kind() {
				case reflect.Bool:
					prop[ukey] = strconv.FormatBool(innerElem.Bool())
					continue

				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					prop[ukey] = strconv.FormatInt(innerElem.Int(), 10)
					continue

				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					prop[ukey] = strconv.FormatUint(innerElem.Uint(), 10)
					continue
				}
			}

			if value := values.Field(i).String(); value != "" {
				prop[ukey] = value
			}
		}

		config := cm.Data["config"]
		cl := strings.Split(config, "\n")
		for i := range cl {
			key := strings.Split(cl[i], "=")
			val, ok := prop[key[0]]
			if ok {
				cl[i] = fmt.Sprintf("%s=%s", key[0], val)
			}
		}

		config = strings.Join(cl, "\n")

		cm.Data["config"] = config
		unstrObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// update the api container envs with result api properties
// as result api server configuration depends on the envs
func updateApiEnv(s v1alpha1.TektonResultSpec) mf.Transformer {
	p := s.ResultsAPIProperties
	return func(u *unstructured.Unstructured) error {

		if u.GetKind() != "Deployment" || u.GetName() != deploymentAPI {
			return nil
		}

		values := reflect.ValueOf(p)
		types := reflect.TypeOf(p)
		prop := make(map[string]string)

		applyLokiStackConfig(prop, s.LokiStackProperties)

		if !s.IsExternalDB {
			prop["DB_HOST"] = "tekton-results-postgres-service." + s.TargetNamespace + ".svc.cluster.local"
		}

		dep := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, dep)
		if err != nil {
			return err
		}
		for i := 0; i < values.NumField(); i++ {
			key := strings.Split(types.Field(i).Tag.Get("json"), ",")[0]
			if key == "" {
				continue
			}
			ukey := strings.ToUpper(key)

			if values.Field(i).Kind() == reflect.Bool {
				prop[ukey] = strconv.FormatBool(values.Field(i).Bool())
				continue
			}

			if values.Field(i).Kind() == reflect.Int64 {
				prop[ukey] = strconv.FormatInt(values.Field(i).Int(), 10)
				continue
			}

			if values.Field(i).Kind() == reflect.Uint64 {
				prop[ukey] = strconv.FormatUint(values.Field(i).Uint(), 10)
				continue
			}

			if values.Field(i).Kind() == reflect.Ptr {
				innerElem := values.Field(i).Elem()

				if !innerElem.IsValid() {
					continue
				}
				switch innerElem.Kind() {
				case reflect.Bool:
					prop[ukey] = strconv.FormatBool(innerElem.Bool())
					continue

				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					prop[ukey] = strconv.FormatInt(innerElem.Int(), 10)
					continue

				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					prop[ukey] = strconv.FormatUint(innerElem.Uint(), 10)
					continue
				}
			}

			if value := values.Field(i).String(); value != "" {
				prop[ukey] = value
			}
		}

		// finds api container and update the matched env and adds other result properties as env
		for containerIndex, container := range dep.Spec.Template.Spec.Containers {
			if container.Name != apiContainerName {
				continue
			}

			existingContainerEnv := container.Env
			if existingContainerEnv == nil {
				existingContainerEnv = make([]corev1.EnvVar, 0)
			}

			replaceEnv(existingContainerEnv, prop)
			for k, v := range prop {
				newEnv := corev1.EnvVar{
					Name:  k,
					Value: v,
				}
				existingContainerEnv = append(existingContainerEnv, newEnv)
			}
			dep.Spec.Template.Spec.Containers[containerIndex].Env = existingContainerEnv
			break
		}

		unstrObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(dep)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

func applyLokiStackConfig(prop map[string]string, lokiProp v1alpha1.LokiStackProperties) {
	if lokiProp.LokiStackName == "" || lokiProp.LokiStackNamespace == "" {
		return
	}
	lokiURL := "https://" + lokiProp.LokiStackName + "-gateway-http" + "." + lokiProp.LokiStackNamespace + ".svc.cluster.local:8080"
	if prop == nil {
		prop = map[string]string{}
	}

	prop[logsAPIKey] = "true"
	prop[logsTypeKey] = "loki"
	prop[loggingProxyPath] = "/api/logs/v1/application"
	prop[loggingAPIURL] = lokiURL
	prop[loggingTokenPath] = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	prop[loggingNamespaceKey] = "kubernetes_namespace_name"
	prop[loggingStaticLabels] = "log_type=application"
	prop[loggingForwarderDelayDuration] = "10"
}

func populateGoogleCreds(props v1alpha1.ResultsAPIProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if props.LogsType != "GCS" || props.GCSCredsSecretName == "" ||
			props.GCSCredsSecretKey == "" || props.LogsAPI == nil ||
			!*props.LogsAPI || u.GetKind() != "Deployment" ||
			u.GetName() != deploymentAPI {
			return nil
		}

		d := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		// find the matching container and add env and secret name object
		for i, container := range d.Spec.Template.Spec.Containers {
			if container.Name != apiContainerName {
				continue
			}
			add := true
			vol := corev1.Volume{
				Name: googleCredsVolName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: props.GCSCredsSecretName,
						Items: []corev1.KeyToPath{{
							Key:  props.GCSCredsSecretKey,
							Path: props.GCSCredsSecretKey,
						}},
					},
				},
			}
			for k := 0; k < len(d.Spec.Template.Spec.Volumes); k++ {
				if d.Spec.Template.Spec.Volumes[k].Name == googleCredsVolName {
					d.Spec.Template.Spec.Volumes[k] = vol
					add = false
				}
			}
			if add {
				d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, vol)
			}

			volMount := corev1.VolumeMount{
				Name:      googleCredsVolName,
				MountPath: googleCredsPath,
			}

			add = true
			for k := 0; k < len(d.Spec.Template.Spec.Containers[i].VolumeMounts); k++ {
				if d.Spec.Template.Spec.Containers[i].VolumeMounts[k].Name == googleCredsVolName {
					d.Spec.Template.Spec.Containers[i].VolumeMounts[k] = volMount
					add = false
				}
			}
			if add {
				d.Spec.Template.Spec.Containers[i].VolumeMounts = append(
					d.Spec.Template.Spec.Containers[i].VolumeMounts, volMount)
			}

			path := googleCredsPath + "/" + props.GCSCredsSecretKey
			newEnv := corev1.EnvVar{
				Name:  googleAPPCredsEnvName,
				Value: path,
			}
			add = true
			for k, env := range d.Spec.Template.Spec.Containers[i].Env {
				if env.Name == googleAPPCredsEnvName {
					d.Spec.Template.Spec.Containers[i].Env[k] = newEnv
					add = false
					break
				}
			}
			if add {
				d.Spec.Template.Spec.Containers[i].Env = append(
					d.Spec.Template.Spec.Containers[i].Env, newEnv)
			}

			break
		}

		uObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(uObj)
		return nil
	}
}

// updates env keys with the secret name into "tekton-results-api" deployment in "api" container
func updateEnvWithSecretName(props v1alpha1.ResultsAPIProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if props.SecretName == "" || u.GetKind() != "Deployment" || u.GetName() != deploymentAPI {
			return nil
		}

		dep := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, dep)
		if err != nil {
			return err
		}

		// find the matching container and add env and secret name object
		for containerIndex, container := range dep.Spec.Template.Spec.Containers {
			if container.Name != apiContainerName {
				continue
			}

			// get existing env from the container
			existingEnv := container.Env
			if existingEnv == nil {
				existingEnv = make([]corev1.EnvVar, 0)
			}

			// update only allowed properties
			for _, propertyKey := range allowedPropertySecretKeys {
				newEnv := corev1.EnvVar{
					Name: propertyKey,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: props.SecretName,
							},
							Key:      propertyKey,
							Optional: ptr.Bool(true),
						},
					},
				}
				// if existing entry found, replace that
				appendNewEnv := true
				for existingIndex, _env := range existingEnv {
					if _env.Name == propertyKey {
						existingEnv[existingIndex] = newEnv
						appendNewEnv = false
						break
					}
				}
				if appendNewEnv {
					existingEnv = append(existingEnv, newEnv)
				}
			}

			// update the changes into the actual container
			dep.Spec.Template.Spec.Containers[containerIndex].Env = existingEnv
			break
		}

		uObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(dep)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(uObj)
		return nil
	}

}

func replaceEnv(envs []corev1.EnvVar, prop map[string]string) {
	for i, env := range envs {
		_, ok := prop[env.Name]
		if ok {
			envs[i].Name = prop[env.Name]
			delete(prop, env.Name)
		}
	}
}

// update api and retention-policy-agent env secret reference with db secret key and name
func updateEnvWithDBSecretName(props v1alpha1.ResultsAPIProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if props.DBSecretName == "" || u.GetKind() != "Deployment" || u.GetName() == resultWatcherDeployment {
			return nil
		}

		dep := &appsv1.Deployment{}
		err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, dep)
		if err != nil {
			return err
		}

		// find the matching container and add env and secret name object
		for containerIndex, container := range dep.Spec.Template.Spec.Containers {

			// get existing env from the container
			existingEnv := container.Env
			if existingEnv == nil {
				existingEnv = make([]corev1.EnvVar, 0)
			}

			// update the value of container keys with provided db secret key
			if props.DBSecretUserKey != "" {
				ContainerEnvKeys[DB_USER] = props.DBSecretUserKey
			}
			if props.DBSecretPasswordKey != "" {
				ContainerEnvKeys[DB_PASSWORD] = props.DBSecretPasswordKey
			}
			for envKey, secretKey := range ContainerEnvKeys {
				var newEnv corev1.EnvVar
				if envKey == DB_USER {
					newEnv = corev1.EnvVar{
						Name: envKey,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: props.DBSecretName,
								},
								Key: secretKey,
							},
						},
					}
				} else {
					newEnv = corev1.EnvVar{
						Name: envKey,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: props.DBSecretName,
								},
								Key: secretKey,
							},
						},
					}
				}
				// if existing entry found, replace that
				appendNewEnv := true
				for existingIndex, _env := range existingEnv {
					if _env.Name == envKey {
						existingEnv[existingIndex] = newEnv
						appendNewEnv = false
						break
					}
				}
				if appendNewEnv {
					existingEnv = append(existingEnv, newEnv)
				}
			}
			// update the changes into the actual container
			dep.Spec.Template.Spec.Containers[containerIndex].Env = existingEnv
			break
		}

		uObj, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(dep)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(uObj)
		return nil
	}
}
