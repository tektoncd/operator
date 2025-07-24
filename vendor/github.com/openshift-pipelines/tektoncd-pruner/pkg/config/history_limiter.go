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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	// tektonprunerv1alpha1 "github.com/openshift-pipelines/tektoncd-pruner/pkg/apis/tektonpruner/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

// HistoryLimiterResourceFuncs defines a set of methods that operate on resources
// with history limit capabilities.
type HistoryLimiterResourceFuncs interface {
	Type() string
	Get(ctx context.Context, namespace, name string) (metav1.Object, error)
	Update(ctx context.Context, resource metav1.Object) error
	Patch(ctx context.Context, namespace, name string, patchBytes []byte) error
	Delete(ctx context.Context, namespace, name string) error
	List(ctx context.Context, namespace, label string) ([]metav1.Object, error)
	GetFailedHistoryLimitCount(namespace, name string, selectors SelectorSpec) (*int32, string)
	GetSuccessHistoryLimitCount(namespace, name string, selectors SelectorSpec) (*int32, string)
	IsSuccessful(resource metav1.Object) bool
	IsFailed(resource metav1.Object) bool
	IsCompleted(resource metav1.Object) bool
	GetDefaultLabelKey() string
	GetEnforcedConfigLevel(namespace, name string, selectors SelectorSpec) EnforcedConfigLevel
}

// HistoryLimiter is a struct that encapsulates functionality for managing resources
// with history limits. It uses the HistoryLimiterResourceFuncs interface to interact
// with different types of resources
type HistoryLimiter struct {
	resourceFn HistoryLimiterResourceFuncs
}

// NewHistoryLimiter creates a new instance of HistoryLimiter, ensuring that the
// provided HistoryLimiterResourceFuncs interface is not nil
func NewHistoryLimiter(resourceFn HistoryLimiterResourceFuncs) (*HistoryLimiter, error) {
	hl := &HistoryLimiter{
		resourceFn: resourceFn,
	}
	if hl.resourceFn == nil {
		return nil, fmt.Errorf("resourceFunc interface can not be nil")
	}

	return hl, nil
}

// ProcessEvent processes an event for a given resource and performs cleanup
// based on its status. The method checks if the resource is in a deletion state,
// whether it has already been processed, and if it's in a completed state. Depending
// on the resource's completion status, it will either trigger cleanup for successful
// or failed resources
func (hl *HistoryLimiter) ProcessEvent(ctx context.Context, resource metav1.Object) error {
	logger := logging.FromContext(ctx)
	logger.Debugw("processing an event for limit logic", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())

	// if the resource is on deletion state, no action needed
	if resource.GetDeletionTimestamp() != nil {
		logger.Debugw("resource is in deletion state", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return nil
	}

	if hl.isProcessed(resource) {
		logger.Debugw("already processed", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return nil
	}

	// if the resource is still in running state, ignore it
	if !hl.resourceFn.IsCompleted(resource) {
		logger.Debugw("resource is not in completion state", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return nil
	}

	defer hl.markAsProcessed(ctx, resource)

	if hl.resourceFn.IsSuccessful(resource) {
		logger.Debugw("success - cleanup", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return hl.DoSuccessfulResourceCleanup(ctx, resource)
	}

	if hl.resourceFn.IsFailed(resource) {
		logger.Debugw("failed - cleanup", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return hl.DoFailedResourceCleanup(ctx, resource)
	}

	return nil
}

// adds an annotation, indicates this resource is already processed
// no action needed on the further reconcile loop for this Resource
// markAsProcessed patches the resource with the annotation 'mark as processed'
func (hl *HistoryLimiter) markAsProcessed(ctx context.Context, resource metav1.Object) {
	logger := logging.FromContext(ctx)

	logger.Debugw("marking resource as processed", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())

	// Fetch the latest version of the resource
	resourceLatest, err := hl.resourceFn.Get(ctx, resource.GetNamespace(), resource.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		logger.Errorw("error getting resource", "resource", hl.resourceFn.Type(),
			"namespace", resource.GetNamespace(), "name", resource.GetName(), zap.Error(err))
		return
	}

	// Prepare the annotation update
	processedTimeAsString := time.Now().Format(time.RFC3339)
	annotations := resourceLatest.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationHistoryLimitCheckProcessed] = processedTimeAsString

	// Create a patch with the new annotations
	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}

	// Convert patchData to JSON
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		logger.Errorw("error marshaling patch data", zap.Error(err))
		return
	}

	// Apply the patch
	err = hl.resourceFn.Patch(ctx, resourceLatest.GetNamespace(), resourceLatest.GetName(), patchBytes)
	if err != nil {
		logger.Errorw("error patching resource with 'mark as processed' annotation",
			"resource", hl.resourceFn.Type(), "namespace", resourceLatest.GetNamespace(), "name", resourceLatest.GetName(), zap.Error(err))
	}
}

func (hl *HistoryLimiter) isProcessed(resource metav1.Object) bool {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, found := annotations[AnnotationHistoryLimitCheckProcessed]
	return found
}

func (hl *HistoryLimiter) DoSuccessfulResourceCleanup(ctx context.Context, resource metav1.Object) error {
	logging := logging.FromContext(ctx)

	logging.Debugw("processing a successful resource", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	return hl.doResourceCleanup(ctx, resource, AnnotationSuccessfulHistoryLimit, hl.resourceFn.GetSuccessHistoryLimitCount, hl.isSuccessfulResource)
}

func (hl *HistoryLimiter) DoFailedResourceCleanup(ctx context.Context, resource metav1.Object) error {
	logging := logging.FromContext(ctx)
	logging.Debugw("processing a failed resource", "resource", hl.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	return hl.doResourceCleanup(ctx, resource, AnnotationFailedHistoryLimit, hl.resourceFn.GetFailedHistoryLimitCount, hl.isFailedResource)
}

func (hl *HistoryLimiter) isFailedResource(resource metav1.Object) bool {
	return hl.resourceFn.IsCompleted(resource) && hl.resourceFn.IsFailed(resource)
}

func (hl *HistoryLimiter) isSuccessfulResource(resource metav1.Object) bool {
	return hl.resourceFn.IsCompleted(resource) && hl.resourceFn.IsSuccessful(resource)
}

func (hl *HistoryLimiter) doResourceCleanup(ctx context.Context, resource metav1.Object, historyLimitAnnotation string, getHistoryLimitFn func(string, string, SelectorSpec) (*int32, string), getResourceFilterFn func(metav1.Object) bool) error {
	logger := logging.FromContext(ctx)

	// get the label key and resource name
	labelKey := getResourceNameLabelKey(resource, hl.resourceFn.GetDefaultLabelKey())
	resourceName := getResourceName(resource, labelKey)

	// Get Annotations and Labels
	resourceAnnotations := resource.GetAnnotations()
	resourceLabels := resource.GetLabels()

	// Construct the selectors with both matchLabels and matchAnnotations
	resourceSelectors := SelectorSpec{}
	if len(resourceAnnotations) > 0 {
		resourceSelectors.MatchAnnotations = resourceAnnotations
	}
	if len(resourceLabels) > 0 {
		resourceSelectors.MatchLabels = resourceLabels
	}

	// Get enforced config level first
	enforcedConfigLevel := hl.resourceFn.GetEnforcedConfigLevel(resource.GetNamespace(), resourceName, resourceSelectors)
	logger.Debugw("enforcedConfigLevel for the resource is", "resourceName", resourceName, "enforcedlevel", enforcedConfigLevel)

	// Get configured history limit
	var historyLimit *int32
	var identifiedBy string
	configHistoryLimit, configIdentifiedBy := getHistoryLimitFn(resource.GetNamespace(), resourceName, resourceSelectors)

	// For resource-level enforcement, check annotation only if it matches config
	annotations := resource.GetAnnotations()
	if enforcedConfigLevel == EnforcedConfigLevelResource && len(annotations) != 0 && annotations[historyLimitAnnotation] != "" {
		annotationLimit, err := strconv.Atoi(annotations[historyLimitAnnotation])
		if err != nil {
			logger.Errorw("error converting history limit annotation to int",
				"resource", hl.resourceFn.Type(),
				"namespace", resource.GetNamespace(),
				"name", resource.GetName(),
				"annotation", historyLimitAnnotation,
				"value", annotations[historyLimitAnnotation],
				zap.Error(err))
			return err
		}
		// Check bounds before converting to int32
		if annotationLimit < 0 || annotationLimit > math.MaxInt32 {
			logger.Errorw("history limit annotation value out of bounds for int32",
				"resource", hl.resourceFn.Type(),
				"namespace", resource.GetNamespace(),
				"name", resource.GetName(),
				"annotation", historyLimitAnnotation,
				"value", annotationLimit)
			return fmt.Errorf("history limit value %d is out of bounds for type int32", annotationLimit)
		}

		// Only use annotation value if it matches configured value
		if configHistoryLimit != nil && annotationLimit == int(*configHistoryLimit) {
			historyLimit = ptr.Int32(int32(annotationLimit))
			identifiedBy = "identifiedBy_resource_ann"
		} else {
			historyLimit = configHistoryLimit
			identifiedBy = configIdentifiedBy
		}
	} else {
		historyLimit = configHistoryLimit
		identifiedBy = configIdentifiedBy
	}

	logger.Debugw("historylimit for the resource", "resourcename", resourceName, "limit", historyLimit, "identifiedBy", identifiedBy)

	if historyLimit == nil || *historyLimit < 0 {
		return nil
	}

	// List Resources (using appropriate selector based on enforcement level and identifier)
	var resources []metav1.Object
	var err error

	if enforcedConfigLevel == EnforcedConfigLevelResource {
		switch identifiedBy {
		case "identifiedBy_resource_name":
			label := fmt.Sprintf("%s=%s", labelKey, resourceName)
			resources, err = hl.resourceFn.List(ctx, resource.GetNamespace(), label)
		case "identifiedBy_resource_ann":
			labelSelector := ""
			for k, v := range resourceAnnotations {
				if labelSelector != "" {
					labelSelector += ","
				}
				labelSelector += fmt.Sprintf("%s=%s", k, v)
			}
			resources, err = hl.resourceFn.List(ctx, resource.GetNamespace(), labelSelector)
		case "identifiedBy_resource_label":
			labelSelector := ""
			for k, v := range resourceLabels {
				if labelSelector != "" {
					labelSelector += ","
				}
				labelSelector += fmt.Sprintf("%s=%s", k, v)
			}
			resources, err = hl.resourceFn.List(ctx, resource.GetNamespace(), labelSelector)
		default:
			resources, err = hl.resourceFn.List(ctx, resource.GetNamespace(), "")
		}
	} else {
		// For namespace or global level, list all resources in namespace
		resources, err = hl.resourceFn.List(ctx, resource.GetNamespace(), "")
	}

	if err != nil {
		return err
	}

	// Filter resources by status (success/failed)
	resourcesFiltered := []metav1.Object{}
	for _, res := range resources {
		if getResourceFilterFn(res) {
			resourcesFiltered = append(resourcesFiltered, res)
		}
	}
	resources = resourcesFiltered

	if int(*historyLimit) > len(resources) {
		return nil
	}

	// Sort resources by creation timestamp (newest first)
	slices.SortStableFunc(resources, func(a, b metav1.Object) int {
		objA := a.GetCreationTimestamp()
		objB := b.GetCreationTimestamp()
		if objA.Time.Before(objB.Time) {
			return 1
		} else if objA.Time.After(objB.Time) {
			return -1
		}
		return 0
	})

	// Select resources to delete (keep newest up to historyLimit)
	var selectionForDeletion []metav1.Object
	if *historyLimit == 0 {
		selectionForDeletion = resources
	} else {
		selectionForDeletion = resources[*historyLimit:]
	}

	// Delete selected resources
	for _, res := range selectionForDeletion {
		logger.Debugw("deleting resource",
			"resource", hl.resourceFn.Type(),
			"namespace", res.GetNamespace(),
			"name", res.GetName(),
			"creationTimestamp", res.GetCreationTimestamp(),
		)
		if err := hl.resourceFn.Delete(ctx, res.GetNamespace(), res.GetName()); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			logger.Errorw("error deleting resource",
				"resource", hl.resourceFn.Type(),
				"namespace", res.GetNamespace(),
				"name", res.GetName(),
				zap.Error(err),
			)
			return err
		}
	}

	return nil
}
