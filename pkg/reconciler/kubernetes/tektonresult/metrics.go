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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"knative.dev/pkg/metrics"
)

var (
	rReconcileCount = stats.Float64("results_reconciled",
		"results reconciled with their log type",
		stats.UnitDimensionless)
	rReconcilerCountView *view.View

	errUninitializedRecorder = fmt.Errorf("ignoring the metrics recording for result failed to initialize the metrics recorder")
)

// Recorder holds keys for Tekton metrics
type Recorder struct {
	initialized bool
	version     tag.Key
	logType     tag.Key
}

// NewRecorder creates a new metrics recorder instance
// to log the PipelineRun related metrics
func NewRecorder() (*Recorder, error) {
	r := &Recorder{
		initialized: true,
	}

	version, err := tag.NewKey("version")
	if err != nil {
		return nil, err
	}
	r.version = version

	logType, err := tag.NewKey("log_type")
	if err != nil {
		return nil, err
	}
	r.logType = logType

	rReconcilerCountView = &view.View{
		Description: rReconcileCount.Description(),
		Measure:     rReconcileCount,
		Aggregation: view.LastValue(),
		TagKeys:     []tag.Key{r.version, r.logType},
	}

	err = view.Register(rReconcilerCountView)

	if err != nil {
		r.initialized = false
		return r, err
	}

	return r, nil
}

// Record the Results reconciled with their log type
func (r *Recorder) Count(version, logType string) error {
	if !r.initialized {
		return errUninitializedRecorder
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.version, version),
		tag.Insert(r.logType, logType),
	)

	if err != nil {
		return err
	}
	metrics.Record(ctx, rReconcileCount.M(float64(1)))
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
