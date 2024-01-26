/*
Copyright 2024 The Tekton Authors

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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestSetAdditionalPACControllerDefault(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"test": {},
				},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults()

	assert.Equal(t, true, *opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Enable)
	assert.Equal(t, "test-pipelines-as-code-configmap", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].ConfigMapName)
	assert.Equal(t, "test-pipelines-as-code-secret", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].SecretName)
}

func TestSetAdditionalPACControllerDefaultHavingAdditionalPACController(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"test": {
						Enable:        ptr.Bool(false),
						ConfigMapName: "test-configmap",
						SecretName:    "test-secret",
						Settings: map[string]string{
							"application-name":    "Additional PACController CI",
							"custom-console-name": "custom",
							"custom-console-url":  "https://custom.com",
						},
					},
				},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults()

	assert.Equal(t, false, *opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Enable)
	assert.Equal(t, "Additional PACController CI", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Settings["application-name"])
	assert.Equal(t, "custom", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Settings["custom-console-name"])
	assert.Equal(t, "https://custom.com", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Settings["custom-console-url"])
}
