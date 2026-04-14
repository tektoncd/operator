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
	"fmt"
	"sync"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var (
	rReconcileGauge metric.Float64Gauge
	once            sync.Once
	errInitMetrics  error

	errUninitializedRecorder = fmt.Errorf("failed to initialize metrics recorder for result")
)

func initMetrics() error {
	meter := otel.Meter("github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonresult")
	var err error
	rReconcileGauge, err = meter.Float64Gauge(
		"tekton_operator_lifecycle_results_reconciled",
		metric.WithDescription("results reconciled with their log type"),
	)
	if err != nil {
		return fmt.Errorf("failed to create results_reconciled gauge: %w", err)
	}
	return nil
}

// Recorder holds keys for Tekton metrics
type Recorder struct {
	initialized bool
}

// NewRecorder creates a new metrics recorder instance
func NewRecorder() (*Recorder, error) {
	once.Do(func() {
		errInitMetrics = initMetrics()
	})
	if errInitMetrics != nil {
		return nil, errInitMetrics
	}

	r := &Recorder{
		initialized: true,
	}
	return r, nil
}

// Count records the Results reconciled with their log type
func (r *Recorder) Count(version, logType string) error {
	if !r.initialized {
		return errUninitializedRecorder
	}

	rReconcileGauge.Record(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("version", version),
			attribute.String("log_type", logType),
		),
	)
	return nil
}

func (m *Recorder) LogMetrics(version string, spec v1alpha1.TektonResultSpec, logger *zap.SugaredLogger) {
	err := m.Count(version, spec.Result.ResultsAPIProperties.LogsType)
	if err != nil {
		logger.Warnf("%v: Failed to log the metrics : %v", v1alpha1.KindTektonResult, err)
	}
}

// RecorderWrapper wraps the existing Recorder to implement this interface.
type RecorderWrapper struct {
	recorder *Recorder
}

// NewRecorderWrapper creates a new RecorderWrapper instance.
func NewRecorderWrapper(recorder *Recorder) *RecorderWrapper {
	return &RecorderWrapper{recorder: recorder}
}

// LogMetrics implements the Metrics interface by converting the provided logType string
// into a TektonResultSpec before calling the underlying Recorder's LogMetrics method.
func (rw *RecorderWrapper) LogMetrics(logType string, version string, logger *zap.SugaredLogger) {
	spec := v1alpha1.TektonResultSpec{
		Result: v1alpha1.Result{
			ResultsAPIProperties: v1alpha1.ResultsAPIProperties{
				LogsType: logType,
			},
		},
	}
	rw.recorder.LogMetrics(version, spec, logger)
}
