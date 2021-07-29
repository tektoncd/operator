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

package common

import (
	"strings"

	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// UpdateDeployments will add a prefix to container image and images in args
// a list of images can be passed to with their name to replace them with any specific image
// a list of tags can be passed so their images will be skipped
// eg. replaceImages := map[string]string{
//		"-shell-image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
//	}
// here `-shell-image` images will be replace by one in map
// skip := []string{
//		"-gsutil-image",
//	}
// here `-gsutil-image` image will be skipped

// UpdateDeployments will also remove runAsUser from container

func UpdateDeployments(prefix string, replaceImg map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i := range d.Spec.Template.Spec.Containers {

			// Prefix Container Image
			c := &d.Spec.Template.Spec.Containers[i]
			c.Image = prefixImage(prefix, c.Image)

			// Prefix Images in Args if there
			for i := 0; i < len(c.Args); i++ {
				val, ok := replaceImg[c.Args[i]]
				if ok {
					c.Args[i+1] = val
					i++
					continue
				}
				c.Args[i] = prefixImage(prefix, c.Args[i])
			}

			// Remove runAsUser
			c.SecurityContext.RunAsUser = nil
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func prefixImage(prefix, img string) string {
	if !strings.Contains(img, ".io/") {
		return img
	}
	arr := strings.Split(strings.Split(img, "@")[0], "/")
	return prefix + arr[len(arr)-1]
}

// RemoveRunAsGroup will remove runAsGroup from all container in a deployment
func RemoveRunAsGroup() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i := range d.Spec.Template.Spec.Containers {
			c := &d.Spec.Template.Spec.Containers[i]
			// Remove runAsGroup
			c.SecurityContext.RunAsGroup = nil
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}
