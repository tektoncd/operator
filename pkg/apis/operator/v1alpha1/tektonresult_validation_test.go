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

func TestTektonResult_ValidateTargetNamespaceDenylist(t *testing.T) {
	tr := &TektonResult{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "result",
			Namespace: "namespace",
		},
		Spec: TektonResultSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "kube-system",
			},
		},
	}

	errs := tr.Validate(context.TODO())
	assert.Equal(t, "invalid value: kube-system: spec.targetNamespace\n'kube-system' is a reserved system namespace and is not allowed", errs.Error())
}

func TestTektonResult_ValidateRouteTLSTermination(t *testing.T) {
	tests := []struct {
		name        string
		termination string
		expectError bool
	}{
		{name: "valid edge", termination: "edge", expectError: false},
		{name: "valid reencrypt", termination: "reencrypt", expectError: false},
		{name: "valid passthrough", termination: "passthrough", expectError: false},
		{name: "empty is valid", termination: "", expectError: false},
		{name: "invalid value", termination: "invalid", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TektonResult{
				ObjectMeta: metav1.ObjectMeta{
					Name: "result",
				},
				Spec: TektonResultSpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "foo",
					},
					Result: Result{
						ResultsAPIProperties: ResultsAPIProperties{
							RouteTLSTermination: tt.termination,
						},
					},
				},
			}
			errs := tr.Validate(context.TODO())
			if tt.expectError {
				assert.Assert(t, errs != nil, "expected validation error for termination %q", tt.termination)
			} else {
				assert.Assert(t, errs == nil, "unexpected validation error for termination %q: %v", tt.termination, errs)
			}
		})
	}
}

func TestTektonResult_ValidateAPISettings(t *testing.T) {
	negative := int64(-1)
	zero := int64(0)
	port := int64(6060)
	maxPort := int64(65535)
	tooBig := int64(65536)
	tests := []struct {
		name        string
		props       ResultsAPIProperties
		expectError string
	}{
		{name: "empty is valid", props: ResultsAPIProperties{}},
		{name: "known sql log level", props: ResultsAPIProperties{SQLLogLevel: "info"}},
		{name: "known tls min version", props: ResultsAPIProperties{TLSMinVersion: "1.2"}},
		{name: "tls alias", props: ResultsAPIProperties{TLSMinVersion: "TLSv1.3"}},
		{name: "zero connection limit", props: ResultsAPIProperties{DBMaxIdleConnections: &zero}},
		{name: "profiling port", props: ResultsAPIProperties{ProfilingPort: &port}},
		{name: "profiling port max", props: ResultsAPIProperties{ProfilingPort: &maxPort}},
		{name: "unknown sql log level", props: ResultsAPIProperties{SQLLogLevel: "debug"}, expectError: "spec.sql_log_level"},
		{name: "unknown tls min version", props: ResultsAPIProperties{TLSMinVersion: "1.4"}, expectError: "spec.tls_min_version"},
		{name: "negative idle connections", props: ResultsAPIProperties{DBMaxIdleConnections: &negative}, expectError: "spec.db_max_idle_connections"},
		{name: "negative open connections", props: ResultsAPIProperties{DBMaxOpenConnections: &negative}, expectError: "spec.db_max_open_connections"},
		{name: "negative grpc pool", props: ResultsAPIProperties{GRPCWorkerPool: &negative}, expectError: "spec.grpc_worker_pool"},
		{name: "negative converter limit", props: ResultsAPIProperties{ConverterDBLimit: &negative}, expectError: "spec.converter_db_limit"},
		{name: "profiling port zero", props: ResultsAPIProperties{ProfilingPort: &zero}, expectError: "spec.profiling_port"},
		{name: "profiling port too big", props: ResultsAPIProperties{ProfilingPort: &tooBig}, expectError: "spec.profiling_port"},
		{name: "profiling port negative", props: ResultsAPIProperties{ProfilingPort: &negative}, expectError: "spec.profiling_port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TektonResult{
				ObjectMeta: metav1.ObjectMeta{Name: "result"},
				Spec: TektonResultSpec{
					CommonSpec: CommonSpec{TargetNamespace: "foo"},
					Result:     Result{ResultsAPIProperties: tt.props},
				},
			}
			errs := tr.Validate(context.TODO())
			if tt.expectError == "" {
				assert.Assert(t, errs == nil, "unexpected validation error: %v", errs)
				return
			}
			assert.Assert(t, errs != nil)
			assert.ErrorContains(t, errs, tt.expectError)
		})
	}
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
