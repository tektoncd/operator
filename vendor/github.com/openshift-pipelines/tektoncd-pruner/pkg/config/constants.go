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

package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// EnvSystemNamespace is the environment variable name used to define the system namespace
	// it is used for setting the namespace where pruner-related resources are managed
	EnvSystemNamespace = "SYSTEM_NAMESPACE"

	// EnvTTLConcurrentWorkersPipelineRun is the environment variable name
	// used to specify the count of concurrent workers in action to prune pipelineruns
	EnvTTLConcurrentWorkersPipelineRun = "TTL_CONCURRENT_WORKERS_PIPELINE_RUN"

	// EnvTTLConcurrentWorkersTaskRun is the environment variable name
	// used to specify the count of concurrent workers in action to prune taskruns
	EnvTTLConcurrentWorkersTaskRun = "TTL_CONCURRENT_WORKERS_TASK_RUN"

	// LabelPipelineName represents the label key in a pipeline run's metadata,
	// where its value corresponds to the name of the pipeline
	LabelPipelineName = "tekton.dev/pipeline"

	// LabelPipelineRunName represents the label key in a pipeline run's metadata,
	// where its value corresponds to the name of the pipeline run
	LabelPipelineRunName = "tekton.dev/pipelineRun"

	// LabelTaskName represents the label key in a task run's metadata,
	// where its value corresponds to the name of the task
	LabelTaskName = "tekton.dev/task"

	// LabelTaskRunName represents the label key in a task run's metadata,
	// where its value corresponds to the name of the task run
	LabelTaskRunName = "tekton.dev/taskRun"

	// KindPipelineRun represents the kind value of pipelineRun custom resource
	KindPipelineRun = "PipelineRun"

	// KindTaskRun represents the kind value of taskRun custom resource
	KindTaskRun = "TaskRun"

	// AnnotationTTLSecondsAfterFinished represents the annotation key
	// that stores the ttlSecondsAfterFinished value for the resource.
	AnnotationTTLSecondsAfterFinished = "pruner.tekton.dev/ttlSecondsAfterFinished"

	// AnnotationResourceNameLabelKey represents the annotation key
	// that stores the label key value used to uniquely identify the resource.
	AnnotationResourceNameLabelKey = "pruner.tekton.dev/resourceNameLabelKey"

	// AnnotationSuccessfulHistoryLimit represents the annotation key
	// that stores the successfulHistoryLimit value for the resource.
	AnnotationSuccessfulHistoryLimit = "pruner.tekton.dev/successfulHistoryLimit"

	// AnnotationFailedHistoryLimit represents the annotation key
	// that stores the failedHistoryLimit value for the resource.
	AnnotationFailedHistoryLimit = "pruner.tekton.dev/failedHistoryLimit"

	// AnnotationHistoryLimitCheckProcessed represents the annotation key
	// that indicates whether history limit checks have been processed for the resource.
	AnnotationHistoryLimitCheckProcessed = "pruner.tekton.dev/historyLimitCheckProcessed"

	// PrunerConfigMapName represents the name of the config map
	// that holds the cluster-wide pruner configuration data
	PrunerConfigMapName = "tekton-pruner-default-spec"

	// PrunerGlobalConfigKey represents the key name
	// used to fetch the cluster-wide pruner configuration data
	PrunerGlobalConfigKey = "global-config"

	// DefaultTTLConcurrentWorkersPipelineRun represents
	// number of workers in the PipelineRun controller
	DefaultTTLConcurrentWorkersPipelineRun = int(5)

	// DefaultTTLConcurrentWorkersTaskRun represents
	// number of workers in the TaskRun controller
	DefaultTTLConcurrentWorkersTaskRun = int(5)

	// DefaultGCInterval represents
	// interval in seconds for the periodic cleanup i.e garbage collector to run
	DefaultPeriodicCleanupIntervalSeconds = 600 // 10 minutes
)

// GetEnvValueAsInt fetches the value of an environment variable and converts it to an integer
// if the environment variable is not set or if the conversion fails, it returns a default value
func GetEnvValueAsInt(envKey string, defaultValue int) (int, error) {
	strValue := os.Getenv(envKey)
	if strValue == "" {
		// If the environment variable is not set, return the default value
		return defaultValue, nil
	}

	// Try to convert the string to an integer
	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		// If conversion fails, return an error with context.
		return 0, fmt.Errorf("failed to convert value of %s to int: %w", envKey, err)
	}

	return intValue, nil
}
