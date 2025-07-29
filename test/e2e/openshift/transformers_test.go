//go:build e2e
// +build e2e

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

package openshift

import (
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
)

// assertPodMountsSystemCAVolume asserts that the Openshift config-trusted-cabundle is
// mounted at the appropriate path for RHEL-based systems.
//
// See openshift documentation for CA mounting details:
//
//	https://github.com/openshift/openshift-docs/blob/a8269cf65696fbd08647c8f3b5d065d53a8a1f52/modules/certificate-injection-using-operators.adoc
func assertPodMountsSystemCAVolume(pod corev1.Pod) error {
	containsVolume := false
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == "config-trusted-system-cabundle-volume" &&
			volume.VolumeSource.ConfigMap != nil &&
			len(volume.VolumeSource.ConfigMap.Items) == 1 &&
			volume.VolumeSource.ConfigMap.LocalObjectReference.Name == "config-trusted-cabundle" &&
			volume.VolumeSource.ConfigMap.Items[0].Key == "ca-bundle.crt" &&
			volume.VolumeSource.ConfigMap.Items[0].Path == "tls-ca-bundle.pem" {
			containsVolume = true
			break
		}
	}

	containsVolumeMount := false
	for _, container := range pod.Spec.Containers {
		for _, volumeMount := range container.VolumeMounts {
			if volumeMount.Name == "config-trusted-system-cabundle-volume" &&
				volumeMount.MountPath == "/etc/pki/ca-trust/extracted/pem" &&
				volumeMount.ReadOnly {
				containsVolumeMount = true
				break
			}
		}
	}
	if !(containsVolume && containsVolumeMount) {
		return fmt.Errorf("Pod %s does not mount the CA bundle at the system CA truste path", pod.Name)
	}
	return nil
}

// TestComponentVolumeMounts verifies the components are created with the appropriate volume mounts for Openshift.
func TestComponentVolumeMounts(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonTrigger:   "trigger",
		TektonAddon:     "addon",
		Namespace:       "",
		TargetNamespace: "tekton-pipelines",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}

	for _, cleanupFunc := range []func(){
		func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) },
	} {
		utils.CleanupOnInterrupt(cleanupFunc)
		defer cleanupFunc()
	}

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	podList, err := clients.KubeClient.CoreV1().Pods(crNames.TargetNamespace).List(t.Context(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to get any pods under the namespace %q: %v", crNames.TargetNamespace, err)
	}
	if len(podList.Items) == 0 {
		t.Fatalf("No pods under the namespace %q found", crNames.TargetNamespace)
	}

	for _, pod := range podList.Items {
		if err = assertPodMountsSystemCAVolume(pod); err != nil {
			t.Fatal(err)
		}
	}

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
