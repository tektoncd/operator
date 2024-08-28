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

	"knative.dev/pkg/ptr"
)

func (tc *TektonChain) SetDefaults(ctx context.Context) {
	tc.Spec.Chain.setDefaults()
}

func (c *Chain) setDefaults() {
	// chains defaults
	if c.ArtifactsTaskRunFormat == "" {
		c.ArtifactsTaskRunFormat = "in-toto"
	}
	if c.ArtifactsTaskRunStorage == nil {
		c.ArtifactsTaskRunStorage = ptr.String("oci")
	}
	if c.ArtifactsPipelineRunFormat == "" {
		c.ArtifactsPipelineRunFormat = "in-toto"
	}
	if c.ArtifactsPipelineRunStorage == nil {
		c.ArtifactsPipelineRunStorage = ptr.String("oci")
	}
	if c.ArtifactsOCIFormat == "" {
		c.ArtifactsOCIFormat = "simplesigning"
	}
	if c.ArtifactsOCIStorage == nil {
		c.ArtifactsOCIStorage = ptr.String("oci")
	}
}
