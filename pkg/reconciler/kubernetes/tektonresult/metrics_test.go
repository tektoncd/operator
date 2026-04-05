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

package tektonresult

import (
	"context"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func resetMetrics() {
	once = sync.Once{}
	errInitMetrics = nil
	rReconcileGauge = nil
}

func TestUninitializedMetrics(t *testing.T) {
	recorder := Recorder{}
	if err := recorder.Count("v0.1", "GCS"); err != errUninitializedRecorder {
		t.Errorf("recorder.Count recording expected to return error %s but got %s", errUninitializedRecorder.Error(), err.Error())
	}
}

func TestMetricsCount(t *testing.T) {
	resetMetrics()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { provider.Shutdown(context.Background()) })

	recorder, err := NewRecorder()
	if err != nil {
		t.Fatalf("failed to initialize recorder: %v", err)
	}

	if err := recorder.Count("v0.1", "GCS"); err != nil {
		t.Fatalf("recorder.Count recording failed: %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "tekton_operator_lifecycle_results_reconciled" {
				found = true
				gauge, ok := m.Data.(metricdata.Gauge[float64])
				if !ok {
					t.Fatalf("expected Gauge[float64], got %T", m.Data)
				}
				if len(gauge.DataPoints) != 1 {
					t.Fatalf("expected 1 data point, got %d", len(gauge.DataPoints))
				}
				dp := gauge.DataPoints[0]
				if dp.Value != 1 {
					t.Errorf("expected value 1, got %v", dp.Value)
				}
				gotAttrs := make(map[string]string)
				for _, kv := range dp.Attributes.ToSlice() {
					gotAttrs[string(kv.Key)] = kv.Value.AsString()
				}
				wantAttrs := map[string]string{"version": "v0.1", "log_type": "GCS"}
				if d := cmp.Diff(wantAttrs, gotAttrs); d != "" {
					t.Errorf("attributes diff (-want, +got): %s", d)
				}
			}
		}
	}
	if !found {
		t.Fatal("results_reconciled metric not found")
	}
}
