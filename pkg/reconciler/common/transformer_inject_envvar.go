/*
Copyright 2025 The Tekton Authors

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

package common

import (
	"os"

	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/version"
)

func DeploymentEnvVarKubernetesMinVersion() mf.Transformer {
	var envVars []corev1.EnvVar
	if minVersion, exists := os.LookupEnv(version.KubernetesMinVersionKey); exists {
		envVars = append(envVars, corev1.EnvVar{
			Name:  version.KubernetesMinVersionKey,
			Value: minVersion,
		})
	}
	return deploymentEnvVars(envVars)
}

func deploymentEnvVars(envVars []corev1.EnvVar) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		if len(envVars) == 0 {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		containers := d.Spec.Template.Spec.Containers
		for i := range containers {
			for _, newEnv := range envVars {
				envVarExists := false

				for j := range containers[i].Env {
					if containers[i].Env[j].Name == newEnv.Name {
						containers[i].Env[j].Value = newEnv.Value
						containers[i].Env[j].ValueFrom = newEnv.ValueFrom
						envVarExists = true
						break
					}
				}
				if !envVarExists {
					containers[i].Env = append(containers[i].Env, newEnv)
				}
			}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}
