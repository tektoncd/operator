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

package tektonpipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var (
	pReconcileCount metric.Int64Counter
	once            sync.Once
	errInitMetrics  error
)

func initMetrics() error {
	meter := otel.Meter("github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpipeline")
	var err error
	pReconcileCount, err = meter.Int64Counter(
		"tekton_operator_lifecycle_pipeline_reconcile_total",
		metric.WithDescription("number of pipeline install"),
	)
	if err != nil {
		return fmt.Errorf("failed to create pipeline_reconcile_count counter: %w", err)
	}
	return nil
}

// Recorder holds keys for Tekton metrics
type Recorder struct {
	initialized     bool
	ReportingPeriod time.Duration
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
		initialized:     true,
		ReportingPeriod: 30 * time.Second,
	}
	return r, nil
}

// Count logs number of times pipeline has been installed or failed to install.
func (r *Recorder) Count(status, version string) error {
	if !r.initialized {
		return fmt.Errorf(
			"failed to initialize metrics recorder for pipeline")
	}

	pReconcileCount.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("version", version),
		),
	)
	return nil
}

func (m *Recorder) LogMetrics(status, version string, logger *zap.SugaredLogger) {
	err := m.Count(status, version)
	if err != nil {
		logger.Warnf("%v: Failed to log the metrics : %v", resourceKind, err)
	}
}
