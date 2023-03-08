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

var allowedArtifactsStorage = sets.NewString("tekton", "oci", "gcs", "docdb", "grafeas", "kafka")

func (tc *TektonChain) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tc.GetName() != ChainResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonChain is allowed by name, %s", ChainResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tc.GetName(), errMsg))
	}

	if tc.Spec.TargetNamespace == "" {
		errs = errs.Also(apis.ErrMissingField("spec.targetNamespace"))
	}

	return errs.Also(tc.Spec.ValidateChainConfig("spec"))
}

func (tcs *TektonChainSpec) ValidateChainConfig(path string) (errs *apis.FieldError) {
	if tcs.ArtifactsTaskRunFormat != "" {
		if tcs.ArtifactsTaskRunFormat != "in-toto" {
			errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsTaskRunFormat, path+".artifacts.taskrun.format"))
		}
	}

	if tcs.ArtifactsTaskRunStorage != "" {
		input := strings.Split(tcs.ArtifactsTaskRunStorage, ",")
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

	if tcs.ArtifactsOCIFormat != "" {
		if tcs.ArtifactsOCIFormat != "simplesigning" {
			errs = errs.Also(apis.ErrInvalidValue(tcs.ArtifactsOCIFormat, path+".artifacts.oci.format"))
		}
	}

	if tcs.ArtifactsOCIStorage != "" {
		input := strings.Split(tcs.ArtifactsOCIStorage, ",")
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

	return errs
}
