/*
Copyright 2022 The Tekton Authors

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
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func Test_ValidateTektonChain_MissingTargetNamespace(t *testing.T) {

	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{},
	}

	err := td.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonChain_OnDelete(t *testing.T) {

	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	err := td.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonChain.Validate() on Delete expected no error, but got one, ValidateTektonChain: %v", err)
	}
}

func Test_ValidateTektonChain_ConfigTaskRunFormat(t *testing.T) {
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsTaskRunFormat = "test"
	err := td.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.artifacts.taskrun.format", err.Error())
}

func Test_ValidateTektonChain_ConfigTaskRunStorage(t *testing.T) {
	value := "test"
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsTaskRunStorage = &value
	err := td.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.artifacts.taskrun.storage", err.Error())
}

func Test_ValidateTektonChain_ConfigTaskRunStorageInvalidValidMix(t *testing.T) {
	value := "tekton, test"
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsTaskRunStorage = &value
	err := td.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.artifacts.taskrun.storage", err.Error())
}

func Test_ValidateTektonChain_ConfigTaskRunStorageValid(t *testing.T) {
	value := "tekton, oci"
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsTaskRunStorage = &value
	err := td.Validate(context.TODO())
	if err != nil {
		t.Errorf("ValidateTektonChain.Validate() expected no error for the given config, but got one, ValidateTektonChain: %v", err)
	}
}

func Test_ValidateTektonChain_ConfigPipelineRunFormat(t *testing.T) {
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsPipelineRunFormat = "test"
	err := td.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.artifacts.pipelinerun.format", err.Error())
}

func Test_ValidateTektonChain_ConfigPipelineRunStorage(t *testing.T) {
	value := "test"
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsPipelineRunStorage = &value
	err := td.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.artifacts.pipelinerun.storage", err.Error())
}

func Test_ValidateTektonChain_ConfigPipelineRunStorageInvalidValidMix(t *testing.T) {
	value := "tekton, test"
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsPipelineRunStorage = &value
	err := td.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.artifacts.pipelinerun.storage", err.Error())
}

func Test_ValidateTektonChain_ConfigPipelineRunStorageValid(t *testing.T) {
	value := "tekton, oci"
	td := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	td.Spec.Chain.ChainProperties.ArtifactsPipelineRunStorage = &value
	err := td.Validate(context.TODO())
	if err != nil {
		t.Errorf("ValidateTektonChain.Validate() expected no error for the given config, but got one, ValidateTektonChain: %v", err)
	}
}

func Test_ValidateTektonChain_ConfigInvalidX509SignerFulcioProvider(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.Spec.Chain.ChainProperties.X509SignerFulcioProvider = "test"
	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.signers.x509.fulcio.provider", err.Error())
}

func Test_ValidateTektonChain_ConfigX509SignerFulcioProvider(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.Spec.Chain.ChainProperties.X509SignerFulcioProvider = "google"
	err := tc.Validate(context.TODO())
	if err != nil {
		t.Errorf("ValidateTektonChain.Validate() expected no error for the given config, but got one, ValidateTektonChain: %v", err)
	}
}

func Test_ValidateTektonChain_ConfigInvalidTransparencyConfigEnabled(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.Spec.Chain.ChainProperties.TransparencyConfigEnabled = "test"
	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.transparency.enabled", err.Error())
}

func Test_ValidateTektonChain_ConfigTransparencyConfigEnabled(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.Spec.Chain.ChainProperties.TransparencyConfigEnabled = "true"
	err := tc.Validate(context.TODO())
	if err != nil {
		t.Errorf("ValidateTektonChain.Validate() expected no error for the given config, but got one, ValidateTektonChain: %v", err)
	}
}

func Test_ValidateTektonChain_InvalidControllerEnv(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.Spec.ControllerEnvs = []corev1.EnvVar{
		{
			Name:  "test",
			Value: "test",
		},
	}
	err := tc.Validate(context.TODO())

	assert.Equal(t, "invalid key name \"test\": supported keys are MONGO_SERVER_URL", err.Error())
}

func Test_ValidateTektonChain_ValidControllerEnv(t *testing.T) {
	tc := &TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain",
			Namespace: "namespace",
		},
		Spec: TektonChainSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	tc.Spec.ControllerEnvs = []corev1.EnvVar{
		{
			Name:  "MONGO_SERVER_URL",
			Value: "test",
		},
	}
	err := tc.Validate(context.TODO())
	if err != nil {
		t.Errorf("ValidateTektonChain: %v", err)
	}
}
