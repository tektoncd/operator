/*
Copyright 2019 The Knative Authors

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
package imagereplacement

import (
	"fmt"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func UpdateDeployment(deploy *appsv1.Deployment, registry *v1alpha1.Registry, log logr.Logger) error {
	containers := deploy.Spec.Template.Spec.Containers
	for index := range containers {
		container := &containers[index]
		log.Info(container.Name)
		newImage := getNewImage(registry, container.Name)
		log.Info(newImage)
		if newImage != "" {
			updateContainerImage(container, newImage, log)
		}

		// replace image in args
		args := container.Args
		for i, v := range args {
			newImage := getNewImage(registry, v)
			if newImage != "" {
				log.Info(fmt.Sprintf("Updating image in Args of container from: %v, to: %v", v, newImage))
				args[i+1] = newImage
			}
		}
	}

	return nil
}

func getNewImage(registry *v1alpha1.Registry, key string) string {
	overrideImage := registry.Override[key]
	if overrideImage != "" {
		return overrideImage
	}
	return ""
}

func updateContainerImage(container *corev1.Container, newImage string, log logr.Logger) {
	log.Info(fmt.Sprintf("Updating container image from: %v, to: %v", container.Image, newImage))
	container.Image = newImage
}
