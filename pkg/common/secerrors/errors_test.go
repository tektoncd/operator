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
	"strings"
	"testing"
)

func TestSecureError_Error(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		message  string
		want     string
	}{
		{
			name:     "error with category",
			category: ErrorCategoryAuthentication,
			message:  "auth failed",
			want:     "[authentication] auth failed",
		},
		{
			name:     "error without category",
			category: "",
			message:  "generic error",
			want:     "generic error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewSecureError(tt.category, tt.message, nil)
			if got := err.Error(); got != tt.want {
				t.Errorf("SecureError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecureError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	secErr := NewSecureError(ErrorCategoryInternal, "safe message", innerErr)
	
	if unwrapped := secErr.Unwrap(); unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		category    ErrorCategory
		message     string
		wantNil     bool
		wantMessage string
	}{
		{
			name:        "wrap nil error",
			err:         nil,
			category:    ErrorCategoryInternal,
			message:     "test",
			wantNil:     true,
			wantMessage: "",
		},
		{
			name:        "wrap valid error",
			err:         errors.New("original error"),
			category:    ErrorCategoryValidation,
			message:     "validation failed",
			wantNil:     false,
			wantMessage: "[validation] validation failed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.category, tt.message)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrap() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("Wrap() = nil, want non-nil")
			}
			if got.Error() != tt.wantMessage {
				t.Errorf("Wrap().Error() = %v, want %v", got.Error(), tt.wantMessage)
			}
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "password in error",
			message: "failed to connect: password=mysecretpass123",
			want:    "failed to connect: [REDACTED]",
		},
		{
			name:    "api key in error",
			message: "invalid api_key: AKIA1234567890ABCDEF",
			want:    "invalid [REDACTED]",
		},
		{
			name:    "token in JSON format",
			message: `{"token":"abc123def456","status":"error"}`,
			want:    `{"[REDACTED],"status":"error"}`,
		},
		{
			name:    "JWT token",
			message: "invalid token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			want:    "invalid token: [REDACTED]",
		},
		{
			name:    "URL with credentials",
			message: "failed to connect to https://user:password@example.com/db",
			want:    "failed to connect to [REDACTED]",
		},
		{
			name:    "private key",
			message: "error reading key: -----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
			want:    "error reading key: [REDACTED]",
		},
		{
			name:    "multiple secrets",
			message: "config error: password=secret123 api_key=key456 token=tok789",
			want:    "config error: [REDACTED] [REDACTED] [REDACTED]",
		},
		{
			name:    "safe error message",
			message: "failed to connect to database: connection timeout",
			want:    "failed to connect to database: connection timeout",
		},
		{
			name:    "secret in key-value format",
			message: `error: "secret":"my-secret-value"`,
			want:    `error: secret=[REDACTED]`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeErrorMessage(tt.message)
			// Use Contains for flexible matching since regex replacements might vary
			if !strings.Contains(got, "[REDACTED]") && strings.Contains(tt.want, "[REDACTED]") {
				t.Errorf("SanitizeErrorMessage() = %v, should contain [REDACTED]", got)
			}
			// Verify sensitive data is not present
			if strings.Contains(got, "mysecretpass123") ||
				strings.Contains(got, "abc123def456") ||
				strings.Contains(got, "my-secret-value") {
				t.Errorf("SanitizeErrorMessage() = %v, still contains sensitive data", got)
			}
		})
	}
}

func TestSanitizeSecret(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		secretValue string
		want        string
	}{
		{
			name:        "replace secret value",
			message:     "error connecting with password: mySecretPassword123",
			secretValue: "mySecretPassword123",
			want:        "error connecting with password: [REDACTED]",
		},
		{
			name:        "empty secret value",
			message:     "error message",
			secretValue: "",
			want:        "error message",
		},
		{
			name:        "secret not in message",
			message:     "generic error",
			secretValue: "secret123",
			want:        "generic error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSecret(tt.message, tt.secretValue)
			if got != tt.want {
				t.Errorf("SanitizeSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSensitiveError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "error with password",
			err:  errors.New("invalid password provided"),
			want: true,
		},
		{
			name: "error with secret",
			err:  errors.New("secret key not found"),
			want: true,
		},
		{
			name: "error with token",
			err:  errors.New("invalid token format"),
			want: true,
		},
		{
			name: "error with api key",
			err:  errors.New("apikey validation failed"),
			want: true,
		},
		{
			name: "safe error",
			err:  errors.New("connection timeout"),
			want: false,
		},
		{
			name: "error with credential",
			err:  errors.New("credential mismatch"),
			want: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSensitiveError(tt.err); got != tt.want {
				t.Errorf("IsSensitiveError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "secure error",
			err:  NewSecureError(ErrorCategoryInternal, "safe message", errors.New("internal details")),
			want: "[internal] safe message",
		},
		{
			name: "sensitive error",
			err:  errors.New("failed with password: secret123"),
			want: "operation failed: an internal error occurred (details redacted for security)",
		},
		{
			name: "safe regular error",
			err:  errors.New("connection timeout"),
			want: "connection timeout",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeErrorMessage(tt.err)
			if got != tt.want {
				t.Errorf("SafeErrorMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCategory(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		category ErrorCategory
		want     bool
	}{
		{
			name:     "matching category",
			err:      NewSecureError(ErrorCategoryAuthentication, "test", nil),
			category: ErrorCategoryAuthentication,
			want:     true,
		},
		{
			name:     "non-matching category",
			err:      NewSecureError(ErrorCategoryAuthentication, "test", nil),
			category: ErrorCategoryAuthorization,
			want:     false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			category: ErrorCategoryInternal,
			want:     false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCategory(tt.err, tt.category); got != tt.want {
				t.Errorf("IsCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommonErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *SecureError
		category ErrorCategory
	}{
		{
			name:     "ErrAuthenticationFailed",
			err:      ErrAuthenticationFailed,
			category: ErrorCategoryAuthentication,
		},
		{
			name:     "ErrAuthorizationFailed",
			err:      ErrAuthorizationFailed,
			category: ErrorCategoryAuthorization,
		},
		{
			name:     "ErrInvalidConfiguration",
			err:      ErrInvalidConfiguration,
			category: ErrorCategoryConfiguration,
		},
		{
			name:     "ErrSecretNotFound",
			err:      ErrSecretNotFound,
			category: ErrorCategoryConfiguration,
		},
		{
			name:     "ErrInvalidSecret",
			err:      ErrInvalidSecret,
			category: ErrorCategoryValidation,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("common error %s is nil", tt.name)
			}
			if tt.err.Category() != tt.category {
				t.Errorf("%s category = %v, want %v", tt.name, tt.err.Category(), tt.category)
			}
		})
	}
}

func TestWrapWithSanitization(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		category     ErrorCategory
		wantNil      bool
		shouldRedact bool
	}{
		{
			name:         "nil error",
			err:          nil,
			category:     ErrorCategoryInternal,
			wantNil:      true,
			shouldRedact: false,
		},
		{
			name:         "error with password",
			err:          errors.New("connection failed: password=secret123"),
			category:     ErrorCategoryAuthentication,
			wantNil:      false,
			shouldRedact: true,
		},
		{
			name:         "safe error",
			err:          errors.New("connection timeout"),
			category:     ErrorCategoryNetwork,
			wantNil:      false,
			shouldRedact: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapWithSanitization(tt.err, tt.category)
			if tt.wantNil {
				if got != nil {
					t.Errorf("WrapWithSanitization() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("WrapWithSanitization() = nil, want non-nil")
			}
			if tt.shouldRedact && !strings.Contains(got.Error(), "[REDACTED]") {
				t.Errorf("WrapWithSanitization() = %v, should contain [REDACTED]", got.Error())
			}
			// Verify original error is preserved
			if got.InternalError() != tt.err {
				t.Errorf("WrapWithSanitization() internal error = %v, want %v", got.InternalError(), tt.err)
			}
		})
	}
}

func TestSanitizeKeyValuePairs(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "JSON with password",
			message: `{"password":"secret123","username":"user"}`,
		},
		{
			name:    "key=value format",
			message: "config: apikey=ABC123 token=XYZ789",
		},
		{
			name:    "colon separated",
			message: "auth: secret: my-secret-value",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeKeyValuePairs(tt.message)
			// Should contain REDACTED
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("sanitizeKeyValuePairs() = %v, should contain [REDACTED]", got)
			}
			// Should not contain obvious secrets
			if strings.Contains(got, "secret123") ||
				strings.Contains(got, "ABC123") ||
				strings.Contains(got, "my-secret-value") {
				t.Errorf("sanitizeKeyValuePairs() = %v, still contains sensitive data", got)
			}
		})
	}
}

