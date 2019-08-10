// +build e2e

/*
Copyright 2019 The Knative Authors
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

package e2e

import (
	optest "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/test"
	"testing"

	"github.com/tektoncd/operator/pkg/apis"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/testgroups"
	_ "github.com/tektoncd/plumbing/scripts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/logstream"
)

// TestConfig verifies the KnativeServing creation, deployment recreation, and KnativeServing deletion.
func TestConfig(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	clients := Setup(t)

	names := test.ResourceNames{
		Config:         test.OperatorName,
		Namespace:      test.OperatorNamespace,
	}

	test.CleanupOnInterrupt(func() { test.TearDown(clients, names) })
	defer test.TearDown(clients, names)
}

func TestPipelineOperator(t *testing.T) {
	initTestingFramework(t)

	// Run test groups (test each CRDs)
	t.Run("config-crd", testgroups.ClusterCRD)
}

func initTestingFramework(t *testing.T) {
	apiVersion := "operator.tekton.dev/v1alpha1"
	kind := "Config"

	configList := &op.ConfigList{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
	}

	if err := optest.AddToFrameworkScheme(apis.AddToScheme, configList); err != nil {
		t.Fatalf("failed to add '%s %s': %v", apiVersion, kind, err)
	}
}
