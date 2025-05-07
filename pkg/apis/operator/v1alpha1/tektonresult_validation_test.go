/*
Copyright 2023 The Tekton Authors

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTektonResult_Validate(t *testing.T) {

	tc := &TektonResult{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-name",
			Namespace: "namespace",
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: wrong-name: metadata.name, Only one instance of TektonResult is allowed by name, result", err.Error())
}

func TestTektonResultWatcherPerformancePropertiesValidate(t *testing.T) {
	tr := &TektonResult{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "result",
			Namespace: "bar",
		},
		Spec: TektonResultSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "foo",
			},
		},
	}

	// Helper to return a pointer to a uint
	getBuckets := func(value uint) *uint {
		return &value
	}

	// Helper to return a pointer to an int32
	getReplicas := func(value int32) *int32 {
		return &value
	}

	statefulsetOrdinals := true

	// --- Validate buckets below the minimum range (0) ---
	tr.Spec.Performance = PerformanceProperties{}
	tr.Spec.Performance.DisableHA = false
	tr.Spec.Performance.Buckets = getBuckets(0)
	errs := tr.Validate(context.TODO())
	assert.Equal(t, "expected 1 <= 0 <= 10: spec.performance.buckets", errs.Error())

	// --- Validate buckets above the maximum range (11) ---
	tr.Spec.Performance = PerformanceProperties{}
	tr.Spec.Performance.DisableHA = false
	tr.Spec.Performance.Buckets = getBuckets(11)
	errs = tr.Validate(context.TODO())
	assert.Equal(t, "expected 1 <= 11 <= 10: spec.performance.buckets", errs.Error())

	// --- Validate valid buckets (minimum valid value: 1) ---
	tr.Spec.Performance = PerformanceProperties{}
	tr.Spec.Performance.DisableHA = false
	tr.Spec.Performance.Buckets = getBuckets(1)
	errs = tr.Validate(context.TODO())
	assert.Equal(t, "", errs.Error())

	// --- Validate valid buckets (maximum valid value: 10) ---
	tr.Spec.Performance = PerformanceProperties{}
	tr.Spec.Performance.DisableHA = false
	tr.Spec.Performance.Buckets = getBuckets(10)
	errs = tr.Validate(context.TODO())
	assert.Equal(t, "", errs.Error())

	// --- Validate valid configuration when StatefulsetOrdinals is true and buckets equal replicas ---
	tr.Spec.Performance = PerformanceProperties{}
	tr.Spec.Performance.DisableHA = false
	bucketValue := uint(5)
	tr.Spec.Performance.Buckets = getBuckets(bucketValue)
	replicaValue := int32(5)
	tr.Spec.Performance.Replicas = getReplicas(replicaValue)
	tr.Spec.Performance.StatefulsetOrdinals = &statefulsetOrdinals
	errs = tr.Validate(context.TODO())
	assert.Equal(t, "", errs.Error())

	// --- Validate error when buckets do not equal replicas while StatefulsetOrdinals is true ---
	tr.Spec.Performance = PerformanceProperties{}
	tr.Spec.Performance.DisableHA = false
	tr.Spec.Performance.StatefulsetOrdinals = &statefulsetOrdinals
	bucketValue = uint(5)
	tr.Spec.Performance.Buckets = getBuckets(bucketValue)
	tr.Spec.Performance.Replicas = getReplicas(3)
	errs = tr.Validate(context.TODO())
	expectedErrorMessage := "invalid value: 3: spec.performance.replicas\n" +
		"spec.performance.replicas must equal spec.performance.buckets for statefulset ordinals"
	assert.Equal(t, expectedErrorMessage, errs.Error())
}
