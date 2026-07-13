/*
Copyright 2026 The Tekton Authors

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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ResultsWatcherProperties defines configuration for the Tekton Results Watcher
// controller. These map to command-line flags on the tekton-results-watcher
// deployment.
type ResultsWatcherProperties struct {
	// Grace period before completed TaskRuns/PipelineRuns are deleted from the
	// cluster after being stored in Results. 0 disables deletion. Negative
	// values delete immediately after completion.
	// +optional
	CompletedRunGracePeriod *metav1.Duration `json:"completed_run_grace_period,omitempty"`

	// When true, resources with owner references are not deleted after the grace
	// period. When false, owner references are ignored for deletion.
	// +optional
	CheckOwner *bool `json:"check_owner,omitempty"`

	// Maximum time to wait for a Run to be stored before clearing the finalizer
	// during deletion.
	// +optional
	StoreDeadline *metav1.Duration `json:"store_deadline,omitempty"`

	// When true, only store Runs after they complete. When false, store Runs on
	// every update throughout their lifecycle.
	// +optional
	DisableStoringIncompleteRuns *bool `json:"disable_storing_incomplete_runs,omitempty"`

	// Enable sending TaskRun/PipelineRun logs to the Results API.
	// +optional
	LogsAPI *bool `json:"logs_api,omitempty"`

	// Collect logs with timestamps.
	// +optional
	LogsTimestamps *bool `json:"logs_timestamps,omitempty"`

	// Store Kubernetes events related to TaskRuns and PipelineRuns.
	// +optional
	StoreEvent *bool `json:"store_event,omitempty"`

	// Comma-separated label keys copied into the Result summary.
	// +optional
	SummaryLabels string `json:"summary_labels,omitempty"`

	// Comma-separated annotation keys copied into the Result summary.
	// +optional
	SummaryAnnotations string `json:"summary_annotations,omitempty"`

	// Label selector for Runs eligible for deletion after the grace period.
	// +optional
	LabelSelector string `json:"label_selector,omitempty"`

	// How long the Watcher waits before reprocessing keys on certain events.
	// +optional
	RequeueInterval *metav1.Duration `json:"requeue_interval,omitempty"`

	// Duration to wait for log forwarder to finish after TaskRun completion.
	// +optional
	ForwardBuffer *metav1.Duration `json:"forward_buffer,omitempty"`

	// Timeout for storing logs before aborting.
	// +optional
	UpdateLogTimeout *metav1.Duration `json:"update_log_timeout,omitempty"`

	// Timeout for the dynamic reconciler to process an event.
	// +optional
	DynamicReconcileTimeout *metav1.Duration `json:"dynamic_reconcile_timeout,omitempty"`

	// Disable updating Tekton CRD annotations during reconcile.
	// +optional
	DisableCRDUpdate *bool `json:"disable_crd_update,omitempty"`
}
