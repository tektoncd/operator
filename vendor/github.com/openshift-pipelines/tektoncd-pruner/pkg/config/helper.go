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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// common functions used across history limiter and ttl handler

func getResourceNameLabelKey(resource metav1.Object, defaultLabelKey string) string {
	annotations := resource.GetAnnotations()
	// update user defined label key
	if len(annotations) > 0 && annotations[AnnotationResourceNameLabelKey] != "" {
		defaultLabelKey = annotations[AnnotationResourceNameLabelKey]
	}

	return defaultLabelKey
}

func getResourceName(resource metav1.Object, labelKey string) string {
	labels := resource.GetLabels()
	// if there is no label present, no option to filter
	if len(labels) == 0 {
		return ""
	}

	// get label value
	return labels[labelKey]
}

/*
// getResourceNameFromMatch returns the resource name for a resource based on annotations first, then labels.
// If all annotations match or if all labels match, it returns the value of the "tekton.dev/pipelineRun" or "tekton.dev/taskRun" label else none
func getResourceNameFromMatch(resource metav1.Object, matchAnnotations map[string]string, matchLabels map[string]string, defaultLabelKey string) string {
	var annotationsMatch bool
	var labelsMatch bool
	var runName string

	labels := resource.GetLabels()

	// If matchAnnotations is not null or empty, process annotations
	if len(matchAnnotations) > 0 {
		annotations := resource.GetAnnotations()
		if len(annotations) > 0 { // Check if the resource has annotations
			// Ensure all annotations in matchAnnotations exist in the resource annotations
			annotationsMatch = true
			for key, value := range matchAnnotations {
				if annotationValue, exists := annotations[key]; !exists || annotationValue != value {
					annotationsMatch = false
					break
				}
			}

			if annotationsMatch {
				if value, exists := labels[defaultLabelKey]; exists {
					runName = value
				}
				return runName
			}
		}
	}

	// If matchLabels is not null or empty, process labels
	if len(matchLabels) > 0 {
		if len(labels) > 0 { // Check if the resource has labels
			// Ensure all labels in matchLabels exist in the resource labels
			labelsMatch = true
			for key, value := range matchLabels {
				if labelValue, exists := labels[key]; !exists || labelValue != value {
					labelsMatch = false
					break
				}
			}

			if labelsMatch {
				if value, exists := labels[defaultLabelKey]; exists {
					runName = value
				}
				return runName
			}
		}
	}

	// If no match found in either annotations or labels, return an empty string
	return ""
}
*/

// Helper function to match labels against label selector
func MatchLabels(labels map[string]string, labelSelector string) bool {
	labelPairs := strings.Split(labelSelector, ",")
	for _, labelPair := range labelPairs {
		parts := strings.Split(labelPair, "=")
		if len(parts) != 2 {
			continue
		}
		labelKey, labelValue := parts[0], parts[1]
		if value, exists := labels[labelKey]; !exists || value != labelValue {
			return false
		}
	}
	return true
}
