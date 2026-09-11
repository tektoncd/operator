# Secure Error and Exception Handling

This document describes the secure error handling practices and utilities available in the Tekton Operator codebase.

## Overview

The `pkg/common/secerrors` package provides comprehensive utilities for handling errors securely, preventing sensitive information leakage through error messages, logs, and Kubernetes events.

## Why Secure Error Handling?

Error messages can inadvertently expose sensitive information such as:
- Passwords, API keys, and tokens
- Database credentials and connection strings
- Internal system paths and stack traces
- Private keys and certificates
- Configuration details that could aid attackers

The secure error handling package automatically sanitizes error messages while preserving debugging information for authorized personnel.

## Core Concepts

### SecureError Type

The `SecureError` type wraps errors with two levels of detail:
- **User Message**: Safe, sanitized message suitable for logs and user display
- **Internal Error**: Full error details (for secure debugging contexts only)

```go
secErr := secerrors.NewSecureError(
    secerrors.ErrorCategoryAuthentication,
    "authentication failed",
    originalError,
)
```

### Error Categories

Errors are categorized for better classification and handling:

- `ErrorCategoryAuthentication` - Authentication failures
- `ErrorCategoryAuthorization` - Authorization/permission issues
- `ErrorCategoryConfiguration` - Configuration errors
- `ErrorCategoryValidation` - Input validation failures
- `ErrorCategoryInternal` - Internal system errors
- `ErrorCategoryNetwork` - Network-related errors
- `ErrorCategoryStorage` - Storage/persistence errors

## Usage Examples

### Basic Error Sanitization

```go
import "github.com/tektoncd/operator/pkg/common/secerrors"

// Sanitize any error message
err := errors.New("connection failed: password=secret123")
safeMsg := secerrors.SafeErrorMessage(err)
// Output: "operation failed: an internal error occurred (details redacted for security)"
```

### Creating Secure Errors

```go
// Wrap an error with a safe message
err := someOperation()
if err != nil {
    return secerrors.Wrap(err, 
        secerrors.ErrorCategoryConfiguration, 
        "configuration validation failed")
}

// Automatically sanitize the error message
secErr := secerrors.WrapWithSanitization(err, secerrors.ErrorCategoryInternal)
```

### Secure Logging

```go
import (
    "github.com/tektoncd/operator/pkg/common/secerrors"
    "knative.dev/pkg/logging"
)

func reconcile(ctx context.Context) error {
    logger := logging.FromContext(ctx)
    
    err := performSensitiveOperation()
    if err != nil {
        // Logs safe message at error level, internal details at debug level
        secerrors.LogError(logger, err, "operation failed")
        return err
    }
    return nil
}
```

### Reconciler Error Handling

```go
import "github.com/tektoncd/operator/pkg/common/secerrors"

func (r *Reconciler) ReconcileKind(ctx context.Context, tc *v1alpha1.TektonConfig) error {
    handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)
    
    // Handle general errors
    if err := r.doSomething(); err != nil {
        return handler.HandleError(err, tc, "configuration update")
    }
    
    // Handle secret-related errors
    secret, err := r.kubeClientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
    if err != nil {
        return handler.HandleSecretError(err, tc, secretName, "fetch")
    }
    
    // Handle authentication errors
    if err := r.authenticate(); err != nil {
        return handler.HandleAuthError(err, tc, "authentication")
    }
    
    return nil
}
```

### Custom Sanitization

```go
// Remove a specific secret value from messages
message := secerrors.SanitizeSecret(errorMsg, secretValue)

// Check if an error contains sensitive information
if secerrors.IsSensitiveError(err) {
    // Handle specially
}
```

## Automatic Sanitization Patterns

The package automatically detects and redacts:

1. **Passwords and Secrets**
   - `password=value`, `secret=value`, `pwd:value`
   - JSON: `{"password": "value"}`

2. **API Keys and Tokens**
   - `api_key=value`, `token=value`, `apikey:value`
   - JWT tokens (eyJ...)

3. **AWS Access Keys**
   - Patterns matching AWS key formats (AKIA...)

4. **Private Keys**
   - PEM-formatted private keys

5. **Credentials in URLs**
   - `https://user:password@host/path`

6. **Base64 Encoded Secrets**
   - Long base64 strings associated with sensitive keywords

All detected patterns are replaced with `[REDACTED]`.

## Best Practices

### DO:

✅ **Use secure error wrappers in reconcilers**
```go
return secerrors.Wrap(err, category, "safe user message")
```

✅ **Use secure logging functions**
```go
secerrors.LogError(logger, err, "operation failed", "resource", resourceName)
```

✅ **Check error categories for conditional handling**
```go
if secerrors.IsCategory(err, secerrors.ErrorCategoryAuthentication) {
    // Handle auth errors specially
}
```

✅ **Use ReconcilerErrorHandler for consistency**
```go
handler := secerrors.NewReconcilerErrorHandler(ctx, recorder)
return handler.HandleError(err, resource, "operation")
```

✅ **Log internal details only at debug level**
```go
// The package automatically logs internal errors at debug level
secerrors.LogError(logger, secErr, "failed")
```

### DON'T:

❌ **Don't use fmt.Errorf with %v for sensitive data**
```go
// BAD: May expose password
return fmt.Errorf("auth failed: %v", err)

// GOOD: Sanitize first
return secerrors.Wrap(err, secerrors.ErrorCategoryAuthentication, "authentication failed")
```

❌ **Don't log raw errors that might contain secrets**
```go
// BAD
logger.Errorf("operation failed: %v", err)

// GOOD
secerrors.LogError(logger, err, "operation failed")
```

❌ **Don't include secrets in error messages**
```go
// BAD
return fmt.Errorf("invalid token: %s", token)

// GOOD
return secerrors.Wrap(err, secerrors.ErrorCategoryValidation, "invalid token format")
```

❌ **Don't expose different error messages for user enumeration**
```go
// BAD: Reveals whether user exists
if userNotFound {
    return errors.New("user not found")
}
if wrongPassword {
    return errors.New("wrong password")
}

// GOOD: Generic message
return secerrors.ErrAuthenticationFailed
```

## Common Secure Errors

Pre-defined errors for common scenarios:

```go
secerrors.ErrAuthenticationFailed     // Generic auth failure
secerrors.ErrAuthorizationFailed      // Permission denied
secerrors.ErrInvalidConfiguration     // Config validation failed
secerrors.ErrSecretNotFound           // Secret not found
secerrors.ErrInvalidSecret            // Secret format invalid
```

## Kubernetes Events

The package integrates with Kubernetes event recording:

```go
// Events automatically use sanitized messages
handler.HandleError(err, resource, "reconciliation")
// Records: "reconciliation failed: [sanitized message]"

// Custom secure events
secerrors.SecureEventf(recorder, resource, corev1.EventTypeWarning, 
    "OperationFailed", "operation failed", err)
```

## Testing

When writing tests, you can verify sanitization:

```go
func TestErrorSanitization(t *testing.T) {
    err := errors.New("failed with password: secret123")
    safeMsg := secerrors.SafeErrorMessage(err)
    
    if strings.Contains(safeMsg, "secret123") {
        t.Error("password not sanitized")
    }
    if !strings.Contains(safeMsg, "[REDACTED]") {
        t.Error("expected redaction marker")
    }
}
```

## Migration Guide

To migrate existing code:

### 1. Replace Direct Error Returns

**Before:**
```go
if err != nil {
    logger.Errorf("failed: %v", err)
    return err
}
```

**After:**
```go
if err != nil {
    secerrors.LogError(logger, err, "operation failed")
    return secerrors.WrapWithSanitization(err, secerrors.ErrorCategoryInternal)
}
```

### 2. Replace fmt.Errorf with Secure Wrappers

**Before:**
```go
return fmt.Errorf("secret operation failed: %v", err)
```

**After:**
```go
return secerrors.Wrap(err, secerrors.ErrorCategoryConfiguration, "secret operation failed")
```

### 3. Use ReconcilerErrorHandler

**Before:**
```go
if err != nil {
    logger.Error("failed", err)
    r.Recorder.Event(resource, corev1.EventTypeWarning, "Failed", err.Error())
    return err
}
```

**After:**
```go
handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)
if err != nil {
    return handler.HandleError(err, resource, "operation")
}
```

## Security Considerations

1. **Debug Logging**: Internal error details are only logged when debug level is enabled
2. **Event Recording**: Kubernetes events always use sanitized messages
3. **Error Wrapping**: Original errors are preserved in the error chain for `errors.Is` and `errors.As`
4. **Performance**: Sanitization uses compiled regex patterns for efficiency

## References

- [OWASP Error Handling](https://cheatsheetseries.owasp.org/cheatsheets/Error_Handling_Cheat_Sheet.html)
- [Go Error Handling Best Practices](https://go.dev/blog/error-handling-and-go)
- [CWE-209: Information Exposure Through an Error Message](https://cwe.mitre.org/data/definitions/209.html)

## Related Jira

- [SRVKP-4185](https://issues.redhat.com/browse/SRVKP-4185) - Threat Model Countermeasures
- [T159](https://redhat.sdelements.com) - Follow best practices for secure error and exception handling

