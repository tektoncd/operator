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
	"strconv"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clockUtil "k8s.io/utils/clock"
	controller "knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	// NoTTL represents a TTL value that indicates no TTL should be applied
	NoTTL = "-1"
	// DefaultTTL represents the default TTL duration if none is specified
	DefaultTTL = 0
)

// TTLResourceFuncs defines the set of functions that should be implemented for
// resources that are subject to Time-To-Live (TTL) management, including determining
// whether a resource is completed, updating or deleting the resource, and handling
// the TTL (time-to-live) after the resource is finished
type TTLResourceFuncs interface {
	Type() string
	Get(ctx context.Context, namespace, name string) (metav1.Object, error)
	Delete(ctx context.Context, namespace, name string) error
	Patch(ctx context.Context, namespace, name string, patchBytes []byte) error
	Update(ctx context.Context, resource metav1.Object) error
	IsCompleted(resource metav1.Object) bool
	GetCompletionTime(resource metav1.Object) (metav1.Time, error)
	Ignore(resource metav1.Object) bool
	GetTTLSecondsAfterFinished(namespace, name string, selectors SelectorSpec) (*int32, string)
	GetDefaultLabelKey() string
	GetEnforcedConfigLevel(namespace, name string, selectors SelectorSpec) EnforcedConfigLevel
}

// TTLHandler is responsible for managing resources with a Time-To-Live (TTL) configuration
type TTLHandler struct {
	clock      clockUtil.Clock // the clock for tracking time
	resourceFn TTLResourceFuncs
}

// NewTTLHandler creates a new instance of TTLHandler, which is responsible for managing
// resources with a Time-To-Live (TTL) configuration and initializes a TTLHandler with
// the provided clock and resource function interface.
func NewTTLHandler(clock clockUtil.Clock, resourceFn TTLResourceFuncs) (*TTLHandler, error) {
	tq := &TTLHandler{
		clock:      clock,
		resourceFn: resourceFn,
	}
	if tq.resourceFn == nil {
		return nil, fmt.Errorf("resourceFunc interface can not be nil")
	}

	if tq.clock == nil {
		tq.clock = clockUtil.RealClock{}
	}

	return tq, nil
}

// ProcessEvent handles an event for a resource by processing its TTL-based actions.
// It evaluates the resource's state, checks whether it should be cleaned up,
// and updates the TTL annotation if needed
func (th *TTLHandler) ProcessEvent(ctx context.Context, resource metav1.Object) error {
	// if a resource is in deletion state, no further action needed
	if resource.GetDeletionTimestamp() != nil {
		return nil
	}

	// if a resource is not completed state, no further action needed
	if !th.resourceFn.IsCompleted(resource) && th.resourceFn.Ignore(resource) {
		return nil
	}

	// update ttl annotation, if not present
	err := th.updateAnnotationTTLSeconds(ctx, resource)
	if err != nil {
		return err
	}

	// if the resource is not available for cleanup, no further action needed
	if !th.needsCleanup(resource) {
		return nil
	}

	return th.removeResource(ctx, resource)
}

// updateAnnotationTTLSeconds updates the TTL annotation of a resource if needed
func (th *TTLHandler) updateAnnotationTTLSeconds(ctx context.Context, resource metav1.Object) error {
	logger := logging.FromContext(ctx)

	// get resource name and selectors first to avoid redundant work if no update needed
	labelKey := getResourceNameLabelKey(resource, th.resourceFn.GetDefaultLabelKey())
	resourceName := getResourceName(resource, labelKey)
	resourceSelectors := th.getResourceSelectors(resource)

	// Check enforced config level early
	enforcedLevel := th.resourceFn.GetEnforcedConfigLevel(resource.GetNamespace(), resourceName, resourceSelectors)
	logger.Debugw("checking TTL configuration",
		"resource", th.resourceFn.Type(),
		"namespace", resource.GetNamespace(),
		"name", resourceName,
		"enforced_level", enforcedLevel)

	// Check if update is needed
	if !th.needsTTLUpdate(resource, enforcedLevel) {
		return nil
	}

	// Get TTL value
	ttl, identifiedBy := th.resourceFn.GetTTLSecondsAfterFinished(resource.GetNamespace(), resourceName, resourceSelectors)
	logger.Debugw("TTL configuration found",
		"ttl", ttl,
		"source", identifiedBy,
		"resource", th.resourceFn.Type(),
		"namespace", resource.GetNamespace(),
		"name", resourceName)

	// Get latest version of resource to avoid conflicts
	resourceLatest, err := th.resourceFn.Get(ctx, resource.GetNamespace(), resource.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get resource: %w", err)
	}

	// Update annotations
	annotations := resourceLatest.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if ttl == nil {
		// If TTL is nil, remove the annotation if it exists
		if _, exists := annotations[AnnotationTTLSecondsAfterFinished]; exists {
			delete(annotations, AnnotationTTLSecondsAfterFinished)
			logger.Debugw("removing TTL annotation - no TTL configuration found",
				"resource", th.resourceFn.Type(),
				"namespace", resource.GetNamespace(),
				"name", resource.GetName())
		}
	} else {
		// Set new TTL annotation
		newTTL := strconv.Itoa(int(*ttl))
		currentTTL, hasCurrentTTL := annotations[AnnotationTTLSecondsAfterFinished]
		if !hasCurrentTTL || currentTTL != newTTL {
			annotations[AnnotationTTLSecondsAfterFinished] = newTTL
			logger.Debugw("updating TTL annotation",
				"resource", th.resourceFn.Type(),
				"namespace", resource.GetNamespace(),
				"name", resource.GetName(),
				"oldTTL", currentTTL,
				"newTTL", newTTL,
				"hadPreviousTTL", hasCurrentTTL)
		} else {
			return nil
		}
	}

	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %w", err)
	}

	if err := th.resourceFn.Patch(ctx, resourceLatest.GetNamespace(), resourceLatest.GetName(), patchBytes); err != nil {
		return fmt.Errorf("failed to patch resource with TTL annotation: %w", err)
	}

	return nil
}

// needsCleanup checks whether a Resource has finished and has a TTL set.
func (th *TTLHandler) needsCleanup(resource metav1.Object) bool {
	// Check completion state first as it's likely to be the most expensive operation
	if !th.resourceFn.IsCompleted(resource) {
		return false
	}

	// get the annotations
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return false
	}

	ttlValue := annotations[AnnotationTTLSecondsAfterFinished]
	return ttlValue != "" && ttlValue != NoTTL
}

// removeResource checks the TTL and deletes the Resource if it has expired
func (th *TTLHandler) removeResource(ctx context.Context, resource metav1.Object) error {
	logger := logging.FromContext(ctx)
	logger.Debugw("checking resource cleanup eligibility",
		"resourceType", th.resourceFn.Type(),
		"namespace", resource.GetNamespace(),
		"name", resource.GetName(),
	)

	// check the resource ttl status
	expiredAt, err := th.processTTL(logger, resource)
	if err != nil {
		return fmt.Errorf("failed to process TTL: %w", err)
	}
	if expiredAt == nil {
		return nil
	}

	// Verify TTL hasn't been modified before deletion
	freshResource, err := th.resourceFn.Get(ctx, resource.GetNamespace(), resource.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get fresh resource: %w", err)
	}

	expiredAt, err = th.processTTL(logger, freshResource)
	if err != nil {
		return fmt.Errorf("failed to process TTL for fresh resource: %w", err)
	}
	if expiredAt == nil {
		return nil
	}

	logger.Debugw("cleaning up expired resource",
		"resourceType", th.resourceFn.Type(),
		"namespace", resource.GetNamespace(),
		"name", resource.GetName(),
		"expiredAt", expiredAt,
	)

	if err := th.resourceFn.Delete(ctx, resource.GetNamespace(), resource.GetName()); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete resource: %w", err)
	}
	return nil
}

// processTTL checks whether a given Resource's TTL has expired, and add it to the queue after the TTL is expected to expire
// if the TTL will expire later.
func (th *TTLHandler) processTTL(logger *zap.SugaredLogger, resource metav1.Object) (expiredAt *time.Time, err error) {
	// We don't care about the Resources that are going to be deleted, or the ones that don't need clean up.
	if resource.GetDeletionTimestamp() != nil || !th.needsCleanup(resource) {
		return nil, nil
	}

	now := th.clock.Now()
	t, e, err := th.timeLeft(logger, resource, &now)
	if err != nil {
		return nil, err
	}

	// TTL has expired
	if *t <= 0 {
		return e, nil
	}

	return nil, th.enqueueAfter(logger, resource, *t)
}

// calculates the remaining time to hold this resource
func (th *TTLHandler) timeLeft(logger *zap.SugaredLogger, resource metav1.Object, since *time.Time) (*time.Duration, *time.Time, error) {
	finishAt, expireAt, err := th.getFinishAndExpireTime(resource)
	if err != nil {
		return nil, nil, err
	}

	if finishAt.After(*since) {
		logger.Warn("found resource finished in the future. This is likely due to time skew in the cluster. Resource cleanup will be deferred.")
	}
	remaining := expireAt.Sub(*since)
	logger.Debugw("resource is in finished state",
		"finishTime", finishAt.UTC(), "remainingTTL", remaining, "startTime", since.UTC(), "deadlineTTL", expireAt.UTC(),
	)
	return &remaining, expireAt, nil
}

// returns finished and expire time of the Resource
func (th *TTLHandler) getFinishAndExpireTime(resource metav1.Object) (*time.Time, *time.Time, error) {
	if !th.needsCleanup(resource) {
		return nil, nil, fmt.Errorf("resource '%s/%s' should not be cleaned up", resource.GetNamespace(), resource.GetName())
	}
	t, err := th.resourceFn.GetCompletionTime(resource)
	if err != nil {
		return nil, nil, err
	}
	finishAt := t.Time
	// get ttl duration
	ttlDuration, err := th.getTTLSeconds(resource)
	if err != nil {
		return nil, nil, err
	}
	expireAt := finishAt.Add(*ttlDuration)
	return &finishAt, &expireAt, nil
}

// returns ttl of the resource
func (th *TTLHandler) getTTLSeconds(resource metav1.Object) (*time.Duration, error) {
	annotations := resource.GetAnnotations()
	// if there is no annotation present, no action needed
	if annotations == nil {
		return nil, nil
	}

	ttlString := annotations[AnnotationTTLSecondsAfterFinished]
	// if there is no ttl present on annotation, no action needed
	if ttlString == "" {
		return nil, nil
	}

	ttl, err := strconv.Atoi(ttlString)
	if err != nil {
		return nil, fmt.Errorf("invalid TTL value %q: %w", ttlString, err)
	}

	// Check for negative TTL values (except -1 which means no TTL)
	if ttl < -1 {
		return nil, fmt.Errorf("TTL value %d must be >= -1", ttl)
	}

	ttlDuration := time.Duration(ttl) * time.Second
	return &ttlDuration, nil
}

// enqueue the Resource for later reconcile
// the resource expire duration is in the future
func (th *TTLHandler) enqueueAfter(logger *zap.SugaredLogger, resource metav1.Object, after time.Duration) error {
	logger.Debugw("the resource to be reconciled later, it has expire in the future",
		"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(), "waitDuration", after,
	)
	return controller.NewRequeueAfter(after)
}

// getResourceSelectors constructs the selector spec for a resource
func (th *TTLHandler) getResourceSelectors(resource metav1.Object) SelectorSpec {
	selectors := SelectorSpec{}
	if annotations := resource.GetAnnotations(); len(annotations) > 0 {
		selectors.MatchAnnotations = annotations
	}
	if labels := resource.GetLabels(); len(labels) > 0 {
		selectors.MatchLabels = labels
	}
	return selectors
}

// needsTTLUpdate determines if a resource needs its TTL annotation updated
func (th *TTLHandler) needsTTLUpdate(resource metav1.Object, enforcedLevel EnforcedConfigLevel) bool {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return true
	}

	currentTTL, exists := annotations[AnnotationTTLSecondsAfterFinished]
	if !exists {
		return true
	}

	// Get the current TTL from config
	labelKey := getResourceNameLabelKey(resource, th.resourceFn.GetDefaultLabelKey())
	resourceName := getResourceName(resource, labelKey)
	resourceSelectors := th.getResourceSelectors(resource)

	configTTL, _ := th.resourceFn.GetTTLSecondsAfterFinished(resource.GetNamespace(), resourceName, resourceSelectors)

	// If there's no config TTL, we should remove the annotation
	if configTTL == nil {
		return true
	}

	// Compare current TTL with config TTL
	configTTLStr := strconv.Itoa(int(*configTTL))
	return currentTTL != configTTLStr
}
