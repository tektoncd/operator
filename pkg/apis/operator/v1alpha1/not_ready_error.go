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

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// conditionsGetter is implemented by the status of every component CR
type conditionsGetter interface {
	GetConditions() apis.Conditions
}

// NotReadyError is returned while a component CR exists but isn't ready yet.
// When the component reports why it isn't ready, the reason is added to the
// error so it shows up in the TektonConfig status instead of a generic
// "reconcile again" message. errors.Is(err, RECONCILE_AGAIN_ERR) holds either way.
func NotReadyError(status conditionsGetter) error {
	if status == nil {
		return RECONCILE_AGAIN_ERR
	}
	reason := notReadyReason(status.GetConditions())
	if reason == "" {
		return RECONCILE_AGAIN_ERR
	}
	return fmt.Errorf("%w: %s", RECONCILE_AGAIN_ERR, reason)
}

// notReadyReason returns the message of the first failed sub condition, which
// carries the actual error, and falls back to the Ready condition message.
func notReadyReason(conditions apis.Conditions) string {
	ready := ""
	for _, c := range conditions {
		if c.Status != corev1.ConditionFalse || c.Message == "" {
			continue
		}
		if c.Type == apis.ConditionReady {
			ready = c.Message
			continue
		}
		return c.Message
	}
	return ready
}
