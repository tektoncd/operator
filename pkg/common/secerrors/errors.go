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
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SecureError is an error type that sanitizes sensitive information before exposing error messages
type SecureError struct {
	// userMessage is the safe message to show to users/logs
	userMessage string
	// internalError is the actual error with potentially sensitive info (for debugging only)
	internalError error
	// category helps classify the error type
	category ErrorCategory
}

// ErrorCategory defines types of errors for better classification
type ErrorCategory string

const (
	// ErrorCategoryAuthentication for auth-related errors
	ErrorCategoryAuthentication ErrorCategory = "authentication"
	// ErrorCategoryAuthorization for authz-related errors
	ErrorCategoryAuthorization ErrorCategory = "authorization"
	// ErrorCategoryConfiguration for config-related errors
	ErrorCategoryConfiguration ErrorCategory = "configuration"
	// ErrorCategoryValidation for validation errors
	ErrorCategoryValidation ErrorCategory = "validation"
	// ErrorCategoryInternal for internal errors
	ErrorCategoryInternal ErrorCategory = "internal"
	// ErrorCategoryNetwork for network-related errors
	ErrorCategoryNetwork ErrorCategory = "network"
	// ErrorCategoryStorage for storage-related errors
	ErrorCategoryStorage ErrorCategory = "storage"
)

// Error implements the error interface and returns the sanitized user message
func (e *SecureError) Error() string {
	if e.category != "" {
		return fmt.Sprintf("[%s] %s", e.category, e.userMessage)
	}
	return e.userMessage
}

// Unwrap returns the underlying error for error chain compatibility
func (e *SecureError) Unwrap() error {
	return e.internalError
}

// InternalError returns the internal error (should only be used for detailed logging in secure contexts)
func (e *SecureError) InternalError() error {
	return e.internalError
}

// Category returns the error category
func (e *SecureError) Category() ErrorCategory {
	return e.category
}

// NewSecureError creates a new secure error with a safe user message
func NewSecureError(category ErrorCategory, userMessage string, internalErr error) *SecureError {
	return &SecureError{
		userMessage:   userMessage,
		internalError: internalErr,
		category:      category,
	}
}

// Wrap wraps an existing error and sanitizes it
func Wrap(err error, category ErrorCategory, userMessage string) *SecureError {
	if err == nil {
		return nil
	}
	return NewSecureError(category, userMessage, err)
}

// WrapWithSanitization wraps an error and automatically sanitizes the message
func WrapWithSanitization(err error, category ErrorCategory) *SecureError {
	if err == nil {
		return nil
	}
	sanitized := SanitizeErrorMessage(err.Error())
	return NewSecureError(category, sanitized, err)
}

// sensitivePatterns contains regex patterns for detecting sensitive information
var sensitivePatterns = []*regexp.Regexp{
	// Passwords
	regexp.MustCompile(`(?i)(password|passwd|pwd)["\s:=]+([^\s"']+)`),
	// API keys and tokens
	regexp.MustCompile(`(?i)(api[_-]?key|token|auth[_-]?token)["\s:=]+([^\s"']+)`),
	// Secret keys
	regexp.MustCompile(`(?i)(secret[_-]?key|secret)["\s:=]+([^\s"']+)`),
	// Private keys (PEM format indicators)
	regexp.MustCompile(`-----BEGIN [A-Z\s]+PRIVATE KEY-----[\s\S]*?-----END [A-Z\s]+PRIVATE KEY-----`),
	// AWS access keys
	regexp.MustCompile(`(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`),
	// Generic credentials in URLs
	regexp.MustCompile(`(?i)([a-z0-9+.-]+:\/\/)([^:@\s]+):([^@\s]+)@`),
	// JWT tokens
	regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
	// Base64 encoded strings that might be secrets (at least 32 chars)
	regexp.MustCompile(`(?i)(secret|password|token|key)["\s:=]+([A-Za-z0-9+/=]{32,})`),
}

// SanitizeErrorMessage removes sensitive information from error messages
func SanitizeErrorMessage(message string) string {
	sanitized := message
	
	for _, pattern := range sensitivePatterns {
		sanitized = pattern.ReplaceAllString(sanitized, "[REDACTED]")
	}
	
	// Additional context-aware sanitization
	sanitized = sanitizeKeyValuePairs(sanitized)
	
	return sanitized
}

// sensitiveKeys are field names that typically contain sensitive data
var sensitiveKeys = []string{
	"password", "passwd", "pwd",
	"secret", "secretkey", "secret_key",
	"token", "authtoken", "auth_token", "apitoken", "api_token",
	"key", "apikey", "api_key", "privatekey", "private_key",
	"credential", "credentials",
	"access_key", "accesskey",
	"auth", "authorization",
}

// sanitizeKeyValuePairs removes sensitive key-value pairs from strings
func sanitizeKeyValuePairs(message string) string {
	result := message
	
	for _, key := range sensitiveKeys {
		// Handle various formats: key=value, key:value, "key":"value", etc.
		patterns := []string{
			fmt.Sprintf(`(?i)%s["\s]*[:=]["\s]*[^\s,}"']+`, key),
			fmt.Sprintf(`(?i)"%s"["\s]*:["\s]*"[^"]*"`, key),
			fmt.Sprintf(`(?i)'%s'["\s]*:["\s]*'[^']*'`, key),
		}
		
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			result = re.ReplaceAllString(result, fmt.Sprintf(`%s=[REDACTED]`, key))
		}
	}
	
	return result
}

// SanitizeSecret replaces a known secret value with [REDACTED] in the message
func SanitizeSecret(message, secretValue string) string {
	if secretValue == "" {
		return message
	}
	return strings.ReplaceAll(message, secretValue, "[REDACTED]")
}

// IsSensitiveError checks if an error might contain sensitive information
func IsSensitiveError(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := strings.ToLower(err.Error())
	
	// Check for sensitive keywords
	for _, key := range sensitiveKeys {
		if strings.Contains(errMsg, key) {
			return true
		}
	}
	
	// Check if it matches any sensitive patterns
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(err.Error()) {
			return true
		}
	}
	
	return false
}

// SafeErrorMessage returns a safe error message for logging/user display
// If the error is sensitive, it returns a generic message
func SafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	
	// Check if it's already a SecureError
	var secErr *SecureError
	if errors.As(err, &secErr) {
		return secErr.Error()
	}
	
	// Check if the error contains sensitive information
	if IsSensitiveError(err) {
		return "operation failed: an internal error occurred (details redacted for security)"
	}
	
	// Sanitize the error message
	return SanitizeErrorMessage(err.Error())
}

// Common secure errors for reuse
var (
	ErrAuthenticationFailed = NewSecureError(
		ErrorCategoryAuthentication,
		"authentication failed",
		errors.New("authentication failed"),
	)
	
	ErrAuthorizationFailed = NewSecureError(
		ErrorCategoryAuthorization,
		"authorization failed: insufficient permissions",
		errors.New("authorization failed"),
	)
	
	ErrInvalidConfiguration = NewSecureError(
		ErrorCategoryConfiguration,
		"invalid configuration",
		errors.New("configuration validation failed"),
	)
	
	ErrSecretNotFound = NewSecureError(
		ErrorCategoryConfiguration,
		"required secret not found",
		errors.New("secret not found"),
	)
	
	ErrInvalidSecret = NewSecureError(
		ErrorCategoryValidation,
		"secret validation failed",
		errors.New("invalid secret format"),
	)
)

// IsCategory checks if an error belongs to a specific category
func IsCategory(err error, category ErrorCategory) bool {
	var secErr *SecureError
	if errors.As(err, &secErr) {
		return secErr.Category() == category
	}
	return false
}

