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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var (
	allowedArtifactsTaskRunFormat                   = sets.NewString("", "in-toto", "slsa/v1", "slsa/v2alpha2", "slsa/v2alpha3")
	allowedArtifactsPipelineRunFormat               = sets.NewString("", "in-toto", "slsa/v1", "slsa/v2alpha2", "slsa/v2alpha3")
	allowedX509SignerFulcioProvider                 = sets.NewString("", "google", "spiffe", "github", "filesystem")
	allowedTransparencyConfigEnabled                = sets.NewString("", "true", "false", "manual")
	allowedArtifactsPipelineRunEnableDeepInspection = sets.NewString("", "true", "false")
	allowedArtifactsStorage                         = sets.NewString("", "tekton", "oci", "gcs", "docdb", "grafeas", "kafka")
	allowedControllerEnvs                           = sets.NewString("MONGO_SERVER_URL")
	allowedBuildDefinitionType                      = sets.NewString("", "https://tekton.dev/chains/v2/slsa", "https://tekton.dev/chains/v2/slsa-tekton")
)

func (tc *TektonChain) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tc.GetName() != ChainResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonChain is allowed by name, %s", ChainResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tc.GetName(), errMsg))
	}

	// execute common spec validations
	errs = errs.Also(tc.Spec.CommonSpec.validate("spec"))

	return errs.Also(tc.Spec.ValidateControllerEnv(), tc.Spec.ValidateChainConfig("spec"))
}

func (tcs *TektonChainSpec) ValidateControllerEnv() (errs *apis.FieldError) {
	if tcs.ControllerEnvs != nil {
		for _, v := range tcs.ControllerEnvs {
			if !allowedControllerEnvs.Has(v.Name) {
				errs = errs.Also(apis.ErrInvalidKeyName(v.Name, fmt.Sprintf("supported keys are %s", strings.Join(allowedControllerEnvs.List(), ","))))
			}
		}
	}
	return errs
}

func (tcs *TektonChainSpec) ValidateChainConfig(path string) (errs *apis.FieldError) {

	if !allowedArtifactsTaskRunFormat.Has(tcs.ArtifactsTaskRunFormat) {
		errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsTaskRunFormat, path+".artifacts.taskrun.format"))
	}

	if tcs.ArtifactsTaskRunStorage != nil {
		input := strings.Split(*tcs.ArtifactsTaskRunStorage, ",")
		for i, v := range input {
			input[i] = strings.TrimSpace(v)
			if !allowedArtifactsStorage.Has(input[i]) {
				errs = errs.Also(apis.ErrInvalidValue(input[i], path+".artifacts.taskrun.storage"))
			}
		}
	}

	if tcs.ArtifactsTaskRunSigner != "" {
		if tcs.ArtifactsTaskRunSigner != "x509" && tcs.ArtifactsTaskRunSigner != "kms" {
			errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsTaskRunSigner, path+".artifacts.taskrun.signer"))
		}
	}

	if !allowedArtifactsPipelineRunFormat.Has(tcs.ArtifactsPipelineRunFormat) {
		errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsPipelineRunFormat, path+".artifacts.pipelinerun.format"))
	}

	if tcs.ArtifactsPipelineRunStorage != nil {
		input := strings.Split(*tcs.ArtifactsPipelineRunStorage, ",")
		for i, v := range input {
			input[i] = strings.TrimSpace(v)
			if !allowedArtifactsStorage.Has(input[i]) {
				errs = errs.Also(apis.ErrInvalidValue(input[i], path+".artifacts.pipelinerun.storage"))
			}
		}
	}

	if tcs.ArtifactsPipelineRunSigner != "" {
		if tcs.ArtifactsPipelineRunSigner != "x509" && tcs.ArtifactsPipelineRunSigner != "kms" {
			errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsPipelineRunSigner, path+".artifacts.pipelinerun.signer"))
		}
	}

	if tcs.ArtifactsOCIFormat != "" {
		if tcs.ArtifactsOCIFormat != "simplesigning" {
			errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsOCIFormat, path+".artifacts.oci.format"))
		}
	}

	if tcs.ArtifactsOCIStorage != nil {
		input := strings.Split(*tcs.ArtifactsOCIStorage, ",")
		for i, v := range input {
			input[i] = strings.TrimSpace(v)
			if !allowedArtifactsStorage.Has(input[i]) {
				errs = errs.Also(apis.ErrInvalidValue(input[i], path+".artifacts.oci.storage"))
			}
		}
	}

	if tcs.ArtifactsOCISigner != "" {
		if tcs.ArtifactsOCISigner != "x509" && tcs.ArtifactsOCISigner != "kms" {
			errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsOCISigner, path+".artifacts.oci.signer"))
		}
	}

	if !allowedX509SignerFulcioProvider.Has(tcs.X509SignerFulcioProvider) {
		errs = errs.Also(apis.ErrInvalidValue(tcs.X509SignerFulcioProvider, path+".signers.x509.fulcio.provider"))
	}

	if !allowedTransparencyConfigEnabled.Has(string(tcs.TransparencyConfigEnabled)) {
		errs = errs.Also(apis.ErrInvalidValue(tcs.TransparencyConfigEnabled, path+".transparency.enabled"))
	}

	if !allowedArtifactsPipelineRunEnableDeepInspection.Has(string(tcs.ArtifactsPipelineRunEnableDeepInspection)) {
		errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsPipelineRunEnableDeepInspection, path+".artifacts.pipelinerun.enable-deep-inspection"))
	}

	if !allowedBuildDefinitionType.Has(tcs.BuildDefinitionBuildType) {
		errs = errs.Also(apis.ErrInvalidValue(tcs.BuildDefinitionBuildType, path+".builddefinition.buildtype"))
	}

	return errs
}
