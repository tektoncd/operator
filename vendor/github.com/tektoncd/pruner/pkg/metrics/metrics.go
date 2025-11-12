/*
Copyright 2025 The Tekton Authors

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

package metrics

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// Metric names
	MetricResourcesProcessed        = "tekton_pruner_controller_resources_processed"
	MetricReconciliationEvents      = "tekton_pruner_controller_reconciliation_events"
	MetricResourcesDeleted          = "tekton_pruner_controller_resources_deleted"
	MetricResourcesErrors           = "tekton_pruner_controller_resources_errors"
	MetricReconciliationDuration    = "tekton_pruner_controller_reconciliation_duration"
	MetricTTLProcessingDuration     = "tekton_pruner_controller_ttl_processing_duration"
	MetricHistoryProcessingDuration = "tekton_pruner_controller_history_processing_duration"
	MetricActiveResourcesCount      = "tekton_pruner_controller_active_resources"
	MetricPendingDeletionsCount     = "tekton_pruner_controller_pending_deletions"
	MetricResourceAgeAtDeletion     = "tekton_pruner_controller_resource_age_at_deletion"

	// Label keys
	LabelNamespace    = "namespace"
	LabelResourceType = "resource_type"
	LabelStatus       = "status"
	LabelReason       = "reason"
	LabelErrorType    = "error_type"
	LabelOperation    = "operation"

	// Label values for resource types
	ResourceTypePipelineRun = "pipelinerun"
	ResourceTypeTaskRun     = "taskrun"

	// Label values for operations
	OperationTTL     = "ttl"
	OperationHistory = "history"

	// Label values for status
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusError   = "error"

	// Label values for error types
	ErrorTypeAPI        = "api_error"
	ErrorTypeTimeout    = "timeout"
	ErrorTypeValidation = "validation"
	ErrorTypeInternal   = "internal"
	ErrorTypeNotFound   = "not_found"
	ErrorTypePermission = "permission"
)

// Recorder holds all the OpenTelemetry instruments for recording metrics
type Recorder struct {
	// Counters
	resourcesProcessed   metric.Int64Counter
	reconciliationEvents metric.Int64Counter
	resourcesDeleted     metric.Int64Counter
	resourcesErrors      metric.Int64Counter

	// Histograms for duration measurements
	reconciliationDuration    metric.Float64Histogram
	ttlProcessingDuration     metric.Float64Histogram
	historyProcessingDuration metric.Float64Histogram
	resourceAgeAtDeletion     metric.Float64Histogram

	// UpDownCounters for gauge-like metrics
	activeResourcesCount  metric.Int64UpDownCounter
	pendingDeletionsCount metric.Int64UpDownCounter

	// Cache for tracking unique resources
	seenResources map[types.UID]bool
	cacheMutex    sync.RWMutex
}

var (
	recorder *Recorder
	once     sync.Once
)

// GetRecorder returns the singleton metrics recorder instance
func GetRecorder() *Recorder {
	once.Do(func() {
		recorder = newRecorder()
	})
	return recorder
}

// newRecorder creates and initializes a new metrics recorder with all instruments
func newRecorder() *Recorder {
	meter := otel.Meter("tekton_pruner_controller")

	r := &Recorder{}

	// Initialize cache for unique resource tracking
	r.seenResources = make(map[types.UID]bool)

	// Initialize counters
	r.resourcesProcessed, _ = meter.Int64Counter(
		MetricResourcesProcessed,
		metric.WithDescription("Total number of Tekton resources processed by the pruner"),
		metric.WithUnit("1"),
	)

	r.reconciliationEvents, _ = meter.Int64Counter(
		MetricReconciliationEvents,
		metric.WithDescription("Total number of reconciliation events processed by the pruner"),
		metric.WithUnit("1"),
	)

	r.resourcesDeleted, _ = meter.Int64Counter(
		MetricResourcesDeleted,
		metric.WithDescription("Total number of Tekton resources deleted by the pruner"),
		metric.WithUnit("1"),
	)

	r.resourcesErrors, _ = meter.Int64Counter(
		MetricResourcesErrors,
		metric.WithDescription("Total number of errors encountered while processing Tekton resources"),
		metric.WithUnit("1"),
	)

	// Initialize histograms
	r.reconciliationDuration, _ = meter.Float64Histogram(
		MetricReconciliationDuration,
		metric.WithDescription("Time spent in reconciliation loops"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0),
	)

	r.ttlProcessingDuration, _ = meter.Float64Histogram(
		MetricTTLProcessingDuration,
		metric.WithDescription("Time spent processing TTL-based pruning"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0),
	)

	r.historyProcessingDuration, _ = meter.Float64Histogram(
		MetricHistoryProcessingDuration,
		metric.WithDescription("Time spent processing history-based pruning"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0),
	)

	r.resourceAgeAtDeletion, _ = meter.Float64Histogram(
		MetricResourceAgeAtDeletion,
		metric.WithDescription("Age of resources when they are deleted"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			60, 300, 600, 1800, 3600, 7200, 14400, 28800, 86400, 172800, 345600, 604800,
		), // 1m, 5m, 10m, 30m, 1h, 2h, 4h, 8h, 1d, 2d, 4d, 1w
	)

	// Initialize up-down counters
	r.activeResourcesCount, _ = meter.Int64UpDownCounter(
		MetricActiveResourcesCount,
		metric.WithDescription("Current number of active Tekton resources being tracked"),
		metric.WithUnit("1"),
	)

	r.pendingDeletionsCount, _ = meter.Int64UpDownCounter(
		MetricPendingDeletionsCount,
		metric.WithDescription("Current number of resources pending deletion"),
		metric.WithUnit("1"),
	)

	return r
}

// Timer represents a duration measurement that can be recorded when stopped
type Timer struct {
	start    time.Time
	recorder *Recorder
	labels   []attribute.KeyValue
}

// NewTimer creates a new timer for measuring durations
func (r *Recorder) NewTimer(labels ...attribute.KeyValue) *Timer {
	return &Timer{
		start:    time.Now(),
		recorder: r,
		labels:   labels,
	}
}

// RecordReconciliationDuration records the duration since the timer was created
func (t *Timer) RecordReconciliationDuration(ctx context.Context) {
	duration := time.Since(t.start).Seconds()
	t.recorder.reconciliationDuration.Record(ctx, duration, metric.WithAttributes(t.labels...))
}

// RecordTTLProcessingDuration records the duration since the timer was created
func (t *Timer) RecordTTLProcessingDuration(ctx context.Context) {
	duration := time.Since(t.start).Seconds()
	t.recorder.ttlProcessingDuration.Record(ctx, duration, metric.WithAttributes(t.labels...))
}

// RecordHistoryProcessingDuration records the duration since the timer was created
func (t *Timer) RecordHistoryProcessingDuration(ctx context.Context) {
	duration := time.Since(t.start).Seconds()
	t.recorder.historyProcessingDuration.Record(ctx, duration, metric.WithAttributes(t.labels...))
}

// RecordReconciliationEvent increments the reconciliation events counter
func (r *Recorder) RecordReconciliationEvent(ctx context.Context, resourceType, namespace, status string) {
	labels := []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
		attribute.String(LabelStatus, status),
	}
	r.reconciliationEvents.Add(ctx, 1, metric.WithAttributes(labels...))
}

// RecordResourceProcessed increments the unique resources counter if this UID hasn't been seen before
func (r *Recorder) RecordResourceProcessed(ctx context.Context, resourceUID types.UID, resourceType, namespace, status string) {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	// Only count if we haven't seen this UID before
	if !r.seenResources[resourceUID] {
		r.seenResources[resourceUID] = true

		labels := []attribute.KeyValue{
			attribute.String(LabelResourceType, resourceType),
			attribute.String(LabelNamespace, namespace),
			attribute.String(LabelStatus, status),
		}
		r.resourcesProcessed.Add(ctx, 1, metric.WithAttributes(labels...))
	}
}

// RecordResourceDeleted increments the resources deleted counter and records age
func (r *Recorder) RecordResourceDeleted(ctx context.Context, resourceType, namespace, operation string, resourceAge time.Duration) {
	// Record deletion count
	labels := []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
		attribute.String(LabelOperation, operation),
	}
	r.resourcesDeleted.Add(ctx, 1, metric.WithAttributes(labels...))

	// Record resource age at deletion
	r.resourceAgeAtDeletion.Record(ctx, resourceAge.Seconds(), metric.WithAttributes(labels...))
}

// RecordResourceError increments the resources error counter
func (r *Recorder) RecordResourceError(ctx context.Context, resourceType, namespace, errorType, reason string) {
	labels := []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
		attribute.String(LabelErrorType, errorType),
		attribute.String(LabelReason, reason),
	}
	r.resourcesErrors.Add(ctx, 1, metric.WithAttributes(labels...))
}

// UpdateActiveResourcesCount updates the active resources gauge
func (r *Recorder) UpdateActiveResourcesCount(ctx context.Context, resourceType, namespace string, delta int64) {
	labels := []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
	}
	r.activeResourcesCount.Add(ctx, delta, metric.WithAttributes(labels...))
}

// UpdatePendingDeletionsCount updates the pending deletions gauge
func (r *Recorder) UpdatePendingDeletionsCount(ctx context.Context, resourceType, namespace string, delta int64) {
	labels := []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
	}
	r.pendingDeletionsCount.Add(ctx, delta, metric.WithAttributes(labels...))
}

// Helper functions for creating common attribute sets

// ResourceAttributes creates common resource-related attributes
func ResourceAttributes(resourceType, namespace string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
	}
}

// ErrorAttributes creates error-related attributes
func ErrorAttributes(resourceType, namespace, errorType, reason string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
		attribute.String(LabelErrorType, errorType),
		attribute.String(LabelReason, reason),
	}
}

// OperationAttributes creates operation-related attributes
func OperationAttributes(resourceType, namespace, operation string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(LabelResourceType, resourceType),
		attribute.String(LabelNamespace, namespace),
		attribute.String(LabelOperation, operation),
	}
}

// ClassifyError determines the error type based on the error
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case isAPIError(err):
		return ErrorTypeAPI
	case isTimeoutError(err):
		return ErrorTypeTimeout
	case isValidationError(err):
		return ErrorTypeValidation
	case isNotFoundError(err):
		return ErrorTypeNotFound
	case isPermissionError(err):
		return ErrorTypePermission
	default:
		return ErrorTypeInternal
	}
}

// Helper functions to classify errors (can be expanded based on actual error types)

func isAPIError(err error) bool {
	// Check for Kubernetes API errors
	return errors.IsInternalError(err) || errors.IsServerTimeout(err) || errors.IsServiceUnavailable(err)
}

func isTimeoutError(err error) bool {
	// Check for timeout-related errors
	return errors.IsTimeout(err) || errors.IsServerTimeout(err)
}

func isValidationError(err error) bool {
	// Check for validation errors
	return errors.IsInvalid(err) || errors.IsBadRequest(err)
}

func isNotFoundError(err error) bool {
	// Check for not found errors
	return errors.IsNotFound(err)
}

func isPermissionError(err error) bool {
	// Check for permission/authorization errors
	return errors.IsForbidden(err) || errors.IsUnauthorized(err)
}
