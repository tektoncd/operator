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
	logger := logging.FromContext(ctx)
	logger.Debugw("processing an event for TTLLOGIC",
		"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(),
	)

	// if a resource is in deletion state, no further action needed
	if resource.GetDeletionTimestamp() != nil {
		logger.Debugw("resource is in deletion state, no action needed",
			"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return nil
	}

	// if a resource is not completed state, no further action needed
	if !th.resourceFn.IsCompleted(resource) && th.resourceFn.Ignore(resource) {
		logger.Debugw("resource is ignored",
			"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName())
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

// updates the TTL of a Resource
func (th *TTLHandler) updateAnnotationTTLSeconds(ctx context.Context, resource metav1.Object) error {
	logger := logging.FromContext(ctx)
	needsUpdate := false
	// get the annotations
	annotations := resource.GetAnnotations()
	if annotations == nil {
		needsUpdate = true
		annotations = map[string]string{}
	}
	if annotations[AnnotationTTLSecondsAfterFinished] == "" {
		needsUpdate = true
	}

	// get resource name, with user defined label key, if not available, go with default label key
	labelKey := getResourceNameLabelKey(resource, th.resourceFn.GetDefaultLabelKey())
	resourceName := getResourceName(resource, labelKey)

	// Construct the selectors with both matchLabels and matchAnnotations
	resourceSelectors := SelectorSpec{}

	// Get Annotations and Labels
	resourceAnnotations := resource.GetAnnotations()
	if len(resourceAnnotations) > 0 {
		resourceSelectors.MatchAnnotations = resourceAnnotations
	}

	resourceLabels := resource.GetLabels()
	if len(resourceLabels) > 0 {
		resourceSelectors.MatchLabels = resourceLabels
	}

	th_enforcedlevel := th.resourceFn.GetEnforcedConfigLevel(resource.GetNamespace(), resourceName, resourceSelectors)
	logger.Debugw("CHECKING-ENFORCED-CONFIG-TTL", "enforced_level", th_enforcedlevel,
		"namespace", resource.GetNamespace(), "name", resourceName, "selectors", resourceSelectors)

	// if the "enforceConfigLevel" is not resource level, do not consider ttl from the resource annotation
	// take it from namespace config or global config
	if th.resourceFn.GetEnforcedConfigLevel(resource.GetNamespace(), resourceName, resourceSelectors) != EnforcedConfigLevelResource {
		needsUpdate = true
	}

	if needsUpdate {
		ttl, _ := th.resourceFn.GetTTLSecondsAfterFinished(resource.GetNamespace(), resourceName, resourceSelectors)
		logger.Debugw("TTL processing", "ttl", ttl, "namespace", resource.GetNamespace(), "name", resourceName, "selectors", resourceSelectors)
		if ttl == nil {
			logger.Debugw("ttl is not defined for this resource, no further action needed",
				"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(),
				"resourceLabelKey", labelKey, "resourceLabelValue", resourceName,
			)
			return nil
		}
		newTTL := strconv.Itoa(int(*ttl))
		previousTTL := annotations[AnnotationTTLSecondsAfterFinished]
		if newTTL == previousTTL {
			// there is no change on the TTL, update action not needed
			return nil
		}
		/*
			resource.SetAnnotations(annotations)
			logger.Debugw("updating ttl of a resource",
				"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(), "ttl", ttl,
			)
			return th.resourceFn.Update(ctx, resource)
		*/
		// Prepare the annotation update
		resourceLatest, err := th.resourceFn.Get(ctx, resource.GetNamespace(), resource.GetName())
		if err != nil {
			if errors.IsNotFound(err) {
				return err
			}
			logger.Errorw("error getting resource", "resource", th.resourceFn.Type(),
				"namespace", resource.GetNamespace(), "name", resource.GetName(), zap.Error(err))
			return err
		}
		annotations := resourceLatest.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[AnnotationTTLSecondsAfterFinished] = newTTL
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
			return err
		}

		// Apply the patch
		err = th.resourceFn.Patch(ctx, resourceLatest.GetNamespace(), resourceLatest.GetName(), patchBytes)
		if err != nil {
			logger.Errorw("error patching resource with 'mark as processed' annotation",
				"resource", th.resourceFn.Type(), "namespace", resourceLatest.GetNamespace(), "name", resourceLatest.GetName(), zap.Error(err))
		}
	}
	return nil
}

// needsCleanup checks whether a Resource has finished and has a TTL set.
func (th *TTLHandler) needsCleanup(resource metav1.Object) bool {
	// get the annotations
	annotations := resource.GetAnnotations()
	// if there is no annotations present, the resource is not available for cleanup
	if annotations == nil {
		return false
	}
	// if there is no ttl present, the resource is not available for cleanup [or]
	// if the ttl is "-1", no further action needed on this Resource
	if annotations[AnnotationTTLSecondsAfterFinished] == "" || annotations[AnnotationTTLSecondsAfterFinished] == "-1" {
		return false
	}

	// if the resource is not in completed state, cleanup not needed
	if !th.resourceFn.IsCompleted(resource) {
		return false
	}

	return true
}

// checks the ttl and deletes the Resource, if the Resource reaches the expire time
func (th *TTLHandler) removeResource(ctx context.Context, resource metav1.Object) error {
	logger := logging.FromContext(ctx)
	logger.Debugw("checking if the resource is ready for cleanup",
		"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(),
	)

	// check the resource ttl status
	expiredAt, err := th.processTTL(logger, resource)
	if err != nil {
		return err
	} else if expiredAt == nil {
		return nil
	}

	// The Resource's TTL is assumed to have expired, but the Resource TTL might be stale.
	// Before deleting the Resource, do a final sanity check.
	// If TTL is modified before we do this check, we cannot be sure if the TTL truly expires.
	// The latest Resource may have a different UID, but it's fine because the checks will be run again.
	freshResource, err := th.resourceFn.Get(ctx, resource.GetNamespace(), resource.GetName())
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// use the latest Resource TTL to see if the TTL truly expires.
	expiredAt, err = th.processTTL(logger, freshResource)
	if err != nil {
		return err
	} else if expiredAt == nil {
		return nil
	}

	// TODO: Cascade deletes the Resources if TTL truly expires.
	// policy := metav1.DeletePropagationForeground
	// options := &client.DeleteOptions{
	// 	PropagationPolicy: &policy,
	// 	Preconditions:     &metav1.Preconditions{UID: &fresh.UID},
	// }
	logger.Debugw("cleaning up a resource",
		"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(),
	)
	err = th.resourceFn.Delete(ctx, resource.GetNamespace(), resource.GetName())
	if err != nil {
		// ignore the error, if the resource is not found
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error("error on removing a resource",
			"resource", th.resourceFn.Type(), "namespace", resource.GetNamespace(), "name", resource.GetName(),
			zap.Error(err),
		)
		return err
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
		return nil, err
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
