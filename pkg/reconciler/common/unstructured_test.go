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

package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNamespacedResource(t *testing.T) {
	got := namespacedResource("v1", "ConfigMap", "testns", "testname")
	want := unstructured.Unstructured{}
	want.SetAPIVersion("v1")
	want.SetKind("ConfigMap")
	want.SetNamespace("testns")
	want.SetName("testname")

	if !equality.Semantic.DeepEqual(got, want) {
		t.Errorf("Got = %v, want %v", got, want)
	}
}

func TestClusterScopedResource(t *testing.T) {
	got := clusterScopedResource("v1", "ConfigMap", "testname")
	want := unstructured.Unstructured{}
	want.SetAPIVersion("v1")
	want.SetKind("ConfigMap")
	want.SetName("testname")

	if !equality.Semantic.DeepEqual(got, want) {
		t.Errorf("Got = %v, want %v", got, want)
	}
}
