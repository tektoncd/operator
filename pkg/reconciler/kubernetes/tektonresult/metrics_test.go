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
	"testing"

	"knative.dev/pkg/metrics/metricstest" // Required to setup metrics env for testing
	_ "knative.dev/pkg/metrics/testing"
)

func TestUninitializedMetrics(t *testing.T) {
	recorder := Recorder{}
	if err := recorder.Count("v0.1", "GCS"); err != errUninitializedRecorder {
		t.Errorf("recorder.Count recording expected to return error %s but got %s", errUninitializedRecorder.Error(), err.Error())
	}
}

func TestMetricsCount(t *testing.T) {

	metricstest.Unregister("results_reconciled")

	recorder, err := NewRecorder()
	if err != nil {
		t.Errorf("failed to initilized recorder, got %s", err.Error())

	}
	if err := recorder.Count("v0.1", "GCS"); err != nil {
		t.Errorf("recorder.Count recording failed got %s", err.Error())
	}
	metricstest.CheckStatsReported(t, "results_reconciled")
	metricstest.CheckLastValueData(t, "results_reconciled", map[string]string{"version": "v0.1", "log_type": "GCS"}, float64(1))
}
