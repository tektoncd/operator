/*
Copyright 2024 The Tekton Authors

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

package secerrors

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// ReconcilerErrorHandler provides secure error handling for reconcilers
type ReconcilerErrorHandler struct {
	logger   *zap.SugaredLogger
	recorder EventRecorder
}

// EventRecorder is an interface for recording Kubernetes events
// This allows us to record events without exposing sensitive information
type EventRecorder interface {
	Event(object interface{}, eventtype, reason, message string)
	Eventf(object interface{}, eventtype, reason, messageFmt string, args ...interface{})
}

// NewReconcilerErrorHandler creates a new error handler for reconcilers
func NewReconcilerErrorHandler(ctx context.Context, recorder EventRecorder) *ReconcilerErrorHandler {
	return &ReconcilerErrorHandler{
		logger:   logging.FromContext(ctx),
		recorder: recorder,
	}
}

// HandleError processes an error with appropriate security sanitization
// Returns a sanitized error suitable for returning from a reconciler
func (h *ReconcilerErrorHandler) HandleError(err error, resource interface{}, operation string) error {
	if err == nil {
		return nil
	}
	
	// Log the error securely
	LogError(h.logger, err, fmt.Sprintf("%s failed", operation))
	
	// Record event with safe message
	if h.recorder != nil {
		safeMsg := SafeErrorMessage(err)
		h.recorder.Event(resource, corev1.EventTypeWarning, "OperationFailed",
			fmt.Sprintf("%s failed: %s", operation, safeMsg))
	}
	
	// Return sanitized error
	return WrapWithSanitization(err, ErrorCategoryInternal)
}

// HandleSecretError handles errors related to secret operations
func (h *ReconcilerErrorHandler) HandleSecretError(err error, resource interface{}, secretName, operation string) error {
	if err == nil {
		return nil
	}
	
	// Create a safe error message that doesn't expose secret details
	safeMsg := fmt.Sprintf("secret operation failed for %s", operation)
	secErr := Wrap(err, ErrorCategoryConfiguration, safeMsg)
	
	// Log securely
	h.logger.Errorw(safeMsg, "secret_name", secretName)
	
	// Record event
	if h.recorder != nil {
		h.recorder.Event(resource, corev1.EventTypeWarning, "SecretOperationFailed", safeMsg)
	}
	
	return secErr
}

// HandleAuthError handles authentication/authorization errors
func (h *ReconcilerErrorHandler) HandleAuthError(err error, resource interface{}, operation string) error {
	if err == nil {
		return nil
	}
	
	// Don't expose auth details
	secErr := Wrap(err, ErrorCategoryAuthentication, "authentication or authorization failed")
	
	LogError(h.logger, secErr, fmt.Sprintf("%s requires authentication", operation))
	
	if h.recorder != nil {
		h.recorder.Event(resource, corev1.EventTypeWarning, "AuthenticationFailed",
			"operation requires valid authentication")
	}
	
	return secErr
}

// HandleValidationError handles validation errors (these are usually safe to expose)
func (h *ReconcilerErrorHandler) HandleValidationError(err error, resource interface{}, field string) error {
	if err == nil {
		return nil
	}
	
	// Validation errors are usually safe, but still sanitize
	safeMsg := SanitizeErrorMessage(err.Error())
	secErr := NewSecureError(ErrorCategoryValidation,
		fmt.Sprintf("validation failed for field %s: %s", field, safeMsg), err)
	
	h.logger.Warnw("validation failed", "field", field, "error", safeMsg)
	
	if h.recorder != nil {
		h.recorder.Eventf(resource, corev1.EventTypeWarning, "ValidationFailed",
			"validation failed for field %s", field)
	}
	
	return secErr
}

// HandleConfigError handles configuration errors
func (h *ReconcilerErrorHandler) HandleConfigError(err error, resource interface{}, configName string) error {
	if err == nil {
		return nil
	}
	
	safeMsg := fmt.Sprintf("configuration error in %s", configName)
	secErr := Wrap(err, ErrorCategoryConfiguration, safeMsg)
	
	LogError(h.logger, secErr, "configuration failed")
	
	if h.recorder != nil {
		h.recorder.Event(resource, corev1.EventTypeWarning, "ConfigurationFailed", safeMsg)
	}
	
	return secErr
}

// WrapReconcilerEvent wraps errors as reconciler events with security sanitization
func WrapReconcilerEvent(err error, eventType, reason string) pkgreconciler.Event {
	if err == nil {
		return nil
	}
	
	safeMsg := SafeErrorMessage(err)
	return &pkgreconciler.ReconcilerEvent{
		EventType: eventType,
		Reason:    reason,
		Format:    "%s",
		Args:      []interface{}{safeMsg},
	}
}

// SecureReconcilerError creates a reconciler event from a secure error
func SecureReconcilerError(secErr *SecureError) pkgreconciler.Event {
	if secErr == nil {
		return nil
	}
	
	eventType := corev1.EventTypeWarning
	reason := "ReconciliationFailed"
	
	// Customize based on error category
	switch secErr.Category() {
	case ErrorCategoryAuthentication, ErrorCategoryAuthorization:
		reason = "AuthenticationFailed"
	case ErrorCategoryConfiguration:
		reason = "ConfigurationFailed"
	case ErrorCategoryValidation:
		reason = "ValidationFailed"
		eventType = corev1.EventTypeWarning
	}
	
	return &pkgreconciler.ReconcilerEvent{
		EventType: eventType,
		Reason:    reason,
		Format:    "%s",
		Args:      []interface{}{secErr.Error()},
	}
}

// GetSanitizedConditionMessage creates a sanitized message for condition status
// This can be used when setting condition messages to ensure no sensitive data leaks
func GetSanitizedConditionMessage(err error, messageFormat string, messageA ...interface{}) string {
	message := fmt.Sprintf(messageFormat, messageA...)
	if err != nil {
		safeMsg := SafeErrorMessage(err)
		if safeMsg != "" {
			message = message + ": " + safeMsg
		}
	}
	return message
}

// SecureEventf records an event with sanitized error information
func SecureEventf(recorder EventRecorder, object interface{}, eventtype, reason, messageFmt string, err error, args ...interface{}) {
	if recorder == nil {
		return
	}
	
	// If error is provided, sanitize it
	if err != nil {
		safeErr := SafeErrorMessage(err)
		args = append(args, safeErr)
		messageFmt = messageFmt + ": %s"
	}
	
	recorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

