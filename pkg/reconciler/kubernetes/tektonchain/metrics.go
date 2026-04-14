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
	"sync"
	"time"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

var (
	rReconcileCount metric.Int64Counter
	once            sync.Once
	errInitMetrics  error
)

func initMetrics() error {
	meter := otel.Meter("github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonchain")
	var err error
	rReconcileCount, err = meter.Int64Counter(
		"tekton_operator_lifecycle_chains_reconciled_total",
		metric.WithDescription("metrics of chains reconciled with labels"),
	)
	if err != nil {
		return fmt.Errorf("failed to create chains_reconciled counter: %w", err)
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

// Count logs when chains is reconciled with version and config labels.
func (r *Recorder) Count(version string, spec v1alpha1.TektonChainSpec) error {
	if !r.initialized {
		return fmt.Errorf(
			"failed to initialize metrics recorder for chains")
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

	rReconcileCount.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("version", version),
			attribute.String("taskrun_format", spec.ArtifactsTaskRunFormat),
			attribute.String("taskrun_storage", taskrunStorage),
			attribute.String("taskrun_signer", spec.ArtifactsTaskRunSigner),
			attribute.String("pipelinerun_format", spec.ArtifactsPipelineRunFormat),
			attribute.String("pipelinerun_storage", pipelinerunStorage),
			attribute.String("pipelinerun_signer", spec.ArtifactsPipelineRunSigner),
			attribute.String("oci_format", spec.ArtifactsOCIFormat),
			attribute.String("oci_storage", ociStorage),
			attribute.String("oci_signer", spec.ArtifactsOCISigner),
		),
	)
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
	err := m.Count(version, newSpec)
	if err != nil {
		logger.Warnf("%v: Failed to log the metrics : %v", resourceKind, err)
	}
}
