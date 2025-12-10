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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	systemCAVolume = "config-trusted-system-cabundle-volume"
	systemCAKey    = "tls-ca-bundle.pem"
	systemCADir    = "/etc/pki/ca-trust/extracted/pem"
)

// ApplyCABundlesToDeployment is a transformer that add the trustedCA volume, mount and
// environment variables so that the deployment uses it.
func ApplyCABundlesToDeployment(u *unstructured.Unstructured) error {
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
	deployment.Spec.Template.Spec.Volumes = common.AddOrReplaceInList(
		deployment.Spec.Template.Spec.Volumes,
		common.NewVolumeWithConfigMap(systemCAVolume, common.TrustedCAConfigMapName, common.TrustedCAKey, systemCAKey),
		func(v corev1.Volume) string { return v.Name },
	)

	// Now that the injected certificates have been added as a volume, let's
	// mount them via volumeMounts in the containers
	for i := range deployment.Spec.Template.Spec.Containers {
		c := deployment.Spec.Template.Spec.Containers[i] // Create a copy of the container
		common.AddCABundlesToContainerVolumes(&c)
		addCABundlesToContainerSystemCAStore(&c)
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
		sts.Spec.Template.Spec.Volumes = common.AddOrReplaceInList(
			sts.Spec.Template.Spec.Volumes,
			common.NewVolumeWithConfigMap(systemCAVolume, common.TrustedCAConfigMapName, common.TrustedCAKey, systemCAKey),
			func(v corev1.Volume) string { return v.Name },
		)

		// Now that the injected certificates have been added as a volume, let's
		// mount them via volumeMounts in the containers
		for i := range sts.Spec.Template.Spec.Containers {
			c := sts.Spec.Template.Spec.Containers[i] // Create a copy of the container
			common.AddCABundlesToContainerVolumes(&c)
			addCABundlesToContainerSystemCAStore(&c)
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

// addCABundlesToContainerSystemCAStore mounts the trusted-ca-configmap into the system ca store.
// This is necessary for components shelling out to "legacy applications" (e.g. cURL or git) to
// use the CA bundles, as "legacy applications"  do not respect SSL_CERT_DIR. In the Openshift
// environment the TrustedCAConfigMap has both the default and custom certificates combined.
// Note that the TrustedCAConfigMap does not contain the Service CA bundle. However that is
// utilized for the internal image registry and its tooling respects SSL_CERT_DIR.
//
// NOTE: This transformer should not be applied to pod templates which could reference
// user-defined images such as a TaskRun or PipelineRun since the transformer both assumes the
// image is a RHEL or a similar environment and because it may override a user's image's custom
// certificate bundle.
//
// See `man(8) update-ca-trust` for documentation on the directory structure and usage
// See openshift documentation for CA mounting details:
//
//	https://github.com/openshift/openshift-docs/blob/a8269cf65696fbd08647c8f3b5d065d53a8a1f52/modules/certificate-injection-using-operators.adoc
func addCABundlesToContainerSystemCAStore(container *corev1.Container) {
	newMount := corev1.VolumeMount{
		Name:      systemCAVolume,
		MountPath: systemCADir,
		ReadOnly:  true,
	}

	container.VolumeMounts = common.AddOrReplaceInList(
		container.VolumeMounts,
		newMount,
		func(v corev1.VolumeMount) string { return v.Name },
	)
}
