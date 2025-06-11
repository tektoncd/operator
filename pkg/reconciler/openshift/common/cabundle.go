/*
Copyright 2020 The Tekton Authors

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
	"encoding/json"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// ApplyCABundles is a transformer that add the trustedCA volume, mount and
// environment variables so that the deployment uses it.
func ApplyCABundles(u *unstructured.Unstructured) error {
	if u.GetKind() != "Deployment" {
		// Don't do anything on something else than Deployment
		return nil
	}

	deployment := &appsv1.Deployment{}
	if err := scheme.Scheme.Convert(u, deployment, nil); err != nil {
		return err
	}

	// Let's add the trusted and service CA bundle ConfigMaps as a volume in
	// the PodSpec which will later be mounted to add certs in the pod.
	deployment.Spec.Template.Spec.Volumes = common.AddCABundleConfigMapsToVolumes(deployment.Spec.Template.Spec.Volumes)

	// Now that the injected certificates have been added as a volume, let's
	// mount them via volumeMounts in the containers
	for i := range deployment.Spec.Template.Spec.Containers {
		c := deployment.Spec.Template.Spec.Containers[i] // Create a copy of the container
		common.AddCABundlesToContainerVolumes(&c)
		deployment.Spec.Template.Spec.Containers[i] = c
	}

	deployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   appsv1.SchemeGroupVersion.Group,
		Version: appsv1.SchemeGroupVersion.Version,
		Kind:    "Deployment",
	})
	m, err := toUnstructured(deployment)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(m.Object)
	return nil
}

func toUnstructured(v interface{}) (*unstructured.Unstructured, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	ud := &unstructured.Unstructured{}
	if err := json.Unmarshal(b, ud); err != nil {
		return nil, err
	}
	return ud, nil
}

// ApplyCABundlesForStatefulSet is a transformer that adds CA bundle configurations to a StatefulSet.
// It configures both trusted CA bundle and service CA bundle by:
// - Adding volumes for the CA bundle ConfigMaps
// - Adding volume mounts to containers
// - Setting up necessary annotations for OpenShift service CA injection
// The function modifies the StatefulSet in place and returns any error encountered.
func ApplyCABundlesForStatefulSet(name string) func(u *unstructured.Unstructured) error {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "StatefulSet" || u.GetName() != name {
			// Don't do anything on something else than the specified StatefulSet
			return nil
		}

		sts := &appsv1.StatefulSet{}
		if err := scheme.Scheme.Convert(u, sts, nil); err != nil {
			return err
		}

		// Let's add the trusted and service CA bundle ConfigMaps as a volume in
		// the PodSpec which will later be mounted to add certs in the pod.
		sts.Spec.Template.Spec.Volumes = common.AddCABundleConfigMapsToVolumes(sts.Spec.Template.Spec.Volumes)

		// Now that the injected certificates have been added as a volume, let's
		// mount them via volumeMounts in the containers
		for i := range sts.Spec.Template.Spec.Containers {
			c := sts.Spec.Template.Spec.Containers[i] // Create a copy of the container
			common.AddCABundlesToContainerVolumes(&c)
			sts.Spec.Template.Spec.Containers[i] = c
		}

		sts.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   appsv1.SchemeGroupVersion.Group,
			Version: appsv1.SchemeGroupVersion.Version,
			Kind:    "StatefulSet",
		})
		m, err := toUnstructured(sts)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(m.Object)
		return nil
	}
}
