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
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

var (
	pReconcileCount = stats.Float64("pipeline_reconcile_count",
		"number of pipeline install",
		stats.UnitDimensionless)
)

const (
	metricsNew     = "NewInstall"
	metricsUpgrade = "Upgrade"
)

// Recorder holds keys for Tekton metrics
type Recorder struct {
	initialized bool
	status      tag.Key
	version     tag.Key

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

	version, err := tag.NewKey("version")
	if err != nil {
		return nil, err
	}
	r.version = version

	err = view.Register(
		&view.View{
			Description: pReconcileCount.Description(),
			Measure:     pReconcileCount,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.status, r.version},
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
func (r *Recorder) Count(status, version string) error {
	if !r.initialized {
		return fmt.Errorf(
			"ignoring the metrics recording for pipelinee failed to initialize the metrics recorder")
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.status, status),
		tag.Insert(r.version, version),
	)

	if err != nil {
		return err
	}

	metrics.Record(ctx, pReconcileCount.M(1))
	return nil
}
