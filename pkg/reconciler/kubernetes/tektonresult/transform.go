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
)

const (
	// Results ConfigMap
	configAPI        = "tekton-results-api-config"
	deploymentAPI    = "tekton-results-api"
	configINFO       = "tekton-results-info"
	configMetrics    = "tekton-results-config-observability"
	configPostgresDB = "tekton-results-postgres"
	pvcLoggingVolume = "tekton-logs"
)

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonResult)
	resultImgs := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ResultsImagePrefix))

	targetNs := comp.GetSpec().GetTargetNamespace()
	extra := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdResults),
		common.ApplyProxySettings,
		common.ReplaceNamespaceInDeploymentArgs(targetNs),
		common.ReplaceNamespaceInDeploymentEnv(targetNs),
		updateApiConfig(instance.Spec.ResultsAPIProperties),
		enablePVCLogging(instance.Spec.ResultsAPIProperties),
		common.AddDeploymentRestrictedPSA(),
		common.AddStatefulSetRestrictedPSA(),
		common.DeploymentImages(resultImgs),
		common.StatefulSetImages(resultImgs),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func enablePVCLogging(p v1alpha1.ResultsAPIProperties) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if p.LoggingPVCName == "" || p.LogsPath == "" || u.GetKind() != "Deployment" || u.GetName() != deploymentAPI {
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

func updateApiConfig(p interface{}) mf.Transformer {
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
				if innerElem.Kind() == reflect.Bool {
					prop[ukey] = strconv.FormatBool(innerElem.Bool())
				} else if innerElem.Kind() == reflect.Uint {
					prop[ukey] = strconv.FormatUint(innerElem.Uint(), 10)
				}
				continue
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
