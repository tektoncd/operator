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

package proxy

import (
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
)

func TestUpdateVolume(t *testing.T) {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{},
			Containers: []v1.Container{
				{
					Name:  "testc",
					Image: "testi",
				},
			},
		},
	}
	podUpdated := updateVolume(pod)
	assert.DeepEqual(t, len(podUpdated.Spec.Containers[0].Env), 1)
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].Env[0].Name, "SSL_CERT_DIR")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].Env[0].Value, "/tekton-custom-certs:/etc/ssl/certs:/etc/pki/tls/certs")

	assert.DeepEqual(t, len(podUpdated.Spec.Volumes), 2)
	assert.DeepEqual(t, podUpdated.Spec.Volumes[0].Name, "config-trusted-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Volumes[0].ConfigMap.Name, "config-trusted-cabundle")
	assert.DeepEqual(t, podUpdated.Spec.Volumes[1].Name, "config-service-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Volumes[1].ConfigMap.Name, "config-service-cabundle")

	assert.DeepEqual(t, len(podUpdated.Spec.Containers[0].VolumeMounts), 2)
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[0].Name, "config-trusted-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[0].SubPath, "ca-bundle.crt")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[1].Name, "config-service-cabundle-volume")
	assert.DeepEqual(t, podUpdated.Spec.Containers[0].VolumeMounts[1].SubPath, "service-ca.crt")
}
