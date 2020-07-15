// +build e2e

/*
Copyright 2020 The Tekton Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    https://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"testing"

	"github.com/tektoncd/operator/test/helpers"
	"github.com/tektoncd/operator/test/tektonpipeline"
	"github.com/tektoncd/operator/test/testgroups"
)

// TestTektonPipeline verifies the TektonPipeline creation, deployment recreation, and TektonPipeline deletion.
func TestTektonPipeline(t *testing.T) {
	clients := helpers.Setup(t)

	names := helpers.ResourceNames{
		TektonPipeline: tektonpipeline.TektonPipelineCRName,
	}

	helpers.CleanupOnInterrupt(func() { helpers.TearDown(t, clients, names) })
	defer helpers.TearDown(t, clients, names)

	// Run the TektonPipeline test
	t.Run("tektonpipeline-cr-test", func(t *testing.T) {
		testgroups.TektonPipelineCRD(t, clients)
	})

	t.Run("tektonaddon-crd", func(t *testing.T) {
		testgroups.AddonCRD(t, clients)
	})
}
