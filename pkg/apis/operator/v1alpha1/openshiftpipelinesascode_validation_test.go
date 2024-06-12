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
	"context"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateAddtionalPACControllerEmptySettings(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "openshift-pipelines",
			},
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"test": {
						ConfigMapName: "test-configmap",
						SecretName:    "test-secret",
					},
				},
			},
		},
	}
	err := opacCR.Validate(context.TODO())
	assert.Error(t, err, "")
}

func TestValidateAddtionalPACControllerInvalidName(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "openshift-pipelines",
			},
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"Test": {
						ConfigMapName: "test-configmap",
						SecretName:    "test-secret",
						Settings: map[string]string{
							"application-name": "Additional PACController CI",
						},
					},
				},
			},
		},
	}
	err := opacCR.Validate(context.TODO())
	assert.Equal(t, fmt.Sprintf("invalid value: invalid resource name %q: must be a valid DNS label: name: spec.additionalPACControllers", "Test"), err.Error())
}

func TestValidateAddtionalPACControllerInvalidConfigMapName(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "openshift-pipelines",
			},
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"test": {
						ConfigMapName: "Test-configmap",
						SecretName:    "test-secret",
						Settings: map[string]string{
							"application-name": "Additional PACController CI",
						},
					},
				},
			},
		},
	}
	err := opacCR.Validate(context.TODO())
	assert.Equal(t, fmt.Sprintf("invalid value: invalid resource name %q: must be a valid DNS label: name: spec.additionalPACControllers.configMapName", "Test-configmap"), err.Error())
}

func TestValidateAddtionalPACControllerInvalidNameLength(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "Openshift-Pipelines",
			},
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"testlengthwhichexceedsthemaximumlength": {
						ConfigMapName: "test-configmap",
						SecretName:    "test-secret",
						Settings: map[string]string{
							"application-name": "Additional PACController CI",
						},
					},
				},
			},
		},
	}
	err := opacCR.Validate(context.TODO())
	assert.Equal(t, fmt.Sprintf("invalid value: invalid resource name %q: length must be no more than 25 characters: name: spec.additionalPACControllers", "testlengthwhichexceedsthemaximumlength"), err.Error())
}

func TestValidateAddtionalPACControllerInvalidSetting(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "Openshift-Pipelines",
			},
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"testlength": {
						ConfigMapName: "test-configmap",
						SecretName:    "test-secret",
						Settings: map[string]string{
							"custom-console-url": "test/path",
							"application-name":   "Additional PACController CI",
						},
					},
				},
			},
		},
	}
	err := opacCR.Validate(context.TODO())
	assert.Equal(t, "invalid value: invalid value: invalid value for URL, error: parse \"test/path\": invalid URI for request: validation failed for field custom-console-url: spec.additionalPACControllers.settings", err.Error())
}
