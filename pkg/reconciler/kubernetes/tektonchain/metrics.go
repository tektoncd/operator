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

package tektonchain

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"knative.dev/pkg/metrics"
)

var (
	rReconcileCount = stats.Float64("chains_reconciled",
		"metrics of chains reconciled with labels",
		stats.UnitDimensionless)
)

// Recorder holds keys for Tekton metrics
type Recorder struct {
	initialized        bool
	version            tag.Key
	taskrunFormat      tag.Key
	taskrunStorage     tag.Key
	taskrunSigner      tag.Key
	pipelinerunFormat  tag.Key
	pipelinerunStorage tag.Key
	pipelinerunSigner  tag.Key
	ociFormat          tag.Key
	ociStorage         tag.Key
	ociSigner          tag.Key

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

	version, err := tag.NewKey("version")
	if err != nil {
		return nil, err
	}
	r.version = version

	taskrunFormat, err := tag.NewKey("taskrun_format")
	if err != nil {
		return nil, err
	}
	r.taskrunFormat = taskrunFormat

	taskrunStorage, err := tag.NewKey("taskrun_storage")
	if err != nil {
		return nil, err
	}
	r.taskrunStorage = taskrunStorage

	taskrunSigner, err := tag.NewKey("taskrun_signer")
	if err != nil {
		return nil, err
	}
	r.taskrunSigner = taskrunSigner

	pipelinerunFormat, err := tag.NewKey("pipelinerun_format")
	if err != nil {
		return nil, err
	}
	r.pipelinerunFormat = pipelinerunFormat

	pipelinerunStorage, err := tag.NewKey("pipelinerun_storage")
	if err != nil {
		return nil, err
	}
	r.pipelinerunStorage = pipelinerunStorage

	pipelinerunSigner, err := tag.NewKey("pipelinerun_signer")
	if err != nil {
		return nil, err
	}
	r.pipelinerunSigner = pipelinerunSigner

	ociFormat, err := tag.NewKey("oci_format")
	if err != nil {
		return nil, err
	}
	r.ociFormat = ociFormat

	ociStorage, err := tag.NewKey("oci_storage")
	if err != nil {
		return nil, err
	}
	r.ociStorage = ociStorage

	ociSigner, err := tag.NewKey("oci_signer")
	if err != nil {
		return nil, err
	}
	r.ociSigner = ociSigner

	err = view.Register(
		&view.View{
			Description: rReconcileCount.Description(),
			Measure:     rReconcileCount,
			Aggregation: view.Count(),
			TagKeys: []tag.Key{r.version,
				r.taskrunFormat, r.taskrunStorage, r.taskrunSigner,
				r.pipelinerunFormat, r.pipelinerunStorage, r.pipelinerunSigner,
				r.ociFormat, r.ociStorage, r.ociSigner},
		},
	)

	if err != nil {
		r.initialized = false
		return r, err
	}

	return r, nil
}

// Logs when chains is reconciled with version and
// config labels.
func (r *Recorder) Count(version string, spec v1alpha1.TektonChainSpec) error {
	if !r.initialized {
		return fmt.Errorf(
			"ignoring the metrics recording for pipelinee failed to initialize the metrics recorder")
	}

	var taskrunStorage, pipelinerunStorage, ociStorage string
	if spec.ArtifactsTaskRunStorage != nil {
		taskrunStorage = *spec.ArtifactsTaskRunStorage
	}
	if spec.ArtifactsPipelineRunStorage != nil {
		pipelinerunStorage = *spec.ArtifactsPipelineRunStorage
	}
	if spec.ArtifactsOCIStorage != nil {
		ociStorage = *spec.ArtifactsOCIStorage
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.version, version),
		tag.Insert(r.taskrunFormat, spec.ArtifactsTaskRunFormat),
		tag.Insert(r.taskrunStorage, taskrunStorage),
		tag.Insert(r.taskrunSigner, spec.ArtifactsTaskRunSigner),
		tag.Insert(r.pipelinerunFormat, spec.ArtifactsPipelineRunFormat),
		tag.Insert(r.pipelinerunStorage, pipelinerunStorage),
		tag.Insert(r.pipelinerunSigner, spec.ArtifactsPipelineRunSigner),
		tag.Insert(r.ociFormat, spec.ArtifactsOCIFormat),
		tag.Insert(r.ociStorage, ociStorage),
		tag.Insert(r.ociSigner, spec.ArtifactsOCISigner),
	)

	if err != nil {
		return err
	}

	metrics.Record(ctx, rReconcileCount.M(1))
	return nil
}

func (m *Recorder) LogMetricsWithSpec(version string, spec v1alpha1.TektonChainSpec, logger *zap.SugaredLogger) {
	err := m.Count(version, spec)
	if err != nil {
		logger.Warnf("%v: Failed to log the metrics : %v", v1alpha1.KindTektonResult, err)
	}
}

func (m *Recorder) LogMetrics(status, version string, logger *zap.SugaredLogger) {
	var newSpec v1alpha1.TektonChainSpec
	err := m.Count(status, newSpec)
	if err != nil {
		logger.Warnf("%v: Failed to log the metrics : %v", resourceKind, err)
	}
}
