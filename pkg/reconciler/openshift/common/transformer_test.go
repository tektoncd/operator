/*
Copyright 2021 The Tekton Authors

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

package common

import (
	"path"
	"reflect"
	"testing"

	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUpdateDeployments(t *testing.T) {
	testData := path.Join("testdata", "test-prefix-images.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	testData = path.Join("testdata", "test-prefix-images-expected.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	pipelinesPrefix := "quay.io/openshift-pipeline/tektoncd-pipeline-"

	replaceImages := map[string]string{
		"-shell-image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
	}
	skip := []string{
		"-gsutil-image",
	}
	newManifest, err := manifest.Transform(UpdateDeployments(pipelinesPrefix, replaceImages, skip))
	assert.NilError(t, err)

	got := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	expected := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("failed to update deployment")
	}
}

func TestUpdateDeploymentsTriggers(t *testing.T) {
	testData := path.Join("testdata", "test-prefix-images-triggers.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	testData = path.Join("testdata", "test-prefix-images-triggers-expected.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	triggersPrefix := "quay.io/openshift-pipeline/tektoncd-triggers-"

	newManifest, err := manifest.Transform(UpdateDeployments(triggersPrefix, map[string]string{}, []string{}))
	assert.NilError(t, err)
	newManifest, err = newManifest.Transform(RemoveRunAsGroup())
	assert.NilError(t, err)

	got := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	expected := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("failed to update deployment")
	}
}

func TestUpdateDeploymentsInterceptor(t *testing.T) {
	testData := path.Join("testdata", "test-prefix-images-interceptor.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	testData = path.Join("testdata", "test-prefix-images-interceptor-expected.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	triggersPrefix := "quay.io/openshift-pipeline/tektoncd-triggers-"

	newManifest, err := manifest.Transform(UpdateDeployments(triggersPrefix, map[string]string{}, []string{}))
	assert.NilError(t, err)
	newManifest, err = newManifest.Transform(RemoveRunAsGroup())
	assert.NilError(t, err)

	got := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	expected := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("failed to update deployment")
	}
}
