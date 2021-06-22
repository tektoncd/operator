/*
Copyright 2019 The Tekton Authors

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

package tektontrigger

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

var (
	tReconcileCount = stats.Float64("trigger_reconcile_count",
		"number of trigger install",
		stats.UnitDimensionless)
)

const (
	metricsSuccess = "success"
	metricsFail    = "fail"
)

// Recorder holds keys for Tekton metrics
type Recorder struct {
	initialized bool
	status      tag.Key

	ReportingPeriod time.Duration
}

// NewRecorder creates a new metrics recorder instance
// to log the PipelineRun related metrics
func NewRecorder() (*Recorder, error) {
	r := &Recorder{
		initialized: true,

		// Default to 30s intervals.
		ReportingPeriod: 30 * time.Second,
	}

	status, err := tag.NewKey("status")
	if err != nil {
		return nil, err
	}
	r.status = status

	err = view.Register(
		&view.View{
			Description: tReconcileCount.Description(),
			Measure:     tReconcileCount,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.status},
		},
	)

	if err != nil {
		r.initialized = false
		return r, err
	}

	return r, nil
}

// Count logs number of times a component (pipeline/trigger atm)
// has been installed or failed to install.
func (r *Recorder) Count(status string) error {
	if !r.initialized {
		return fmt.Errorf(
			"ignoring the metrics recording for trigger , failed to initialize the metrics recorder")
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.status, status),
	)

	if err != nil {
		return err
	}

	metrics.Record(ctx, tReconcileCount.M(1))
	return nil
}
