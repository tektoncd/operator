# Package secerrors

Package `secerrors` provides secure error handling utilities for the Tekton Operator, preventing sensitive information leakage through error messages, logs, and Kubernetes events.

## Quick Start

```go
import "github.com/tektoncd/operator/pkg/common/secerrors"

func (r *Reconciler) ReconcileKind(ctx context.Context, resource *v1alpha1.TektonHub) error {
    handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)
    
    // Handle errors securely
    if err := someOperation(); err != nil {
        return handler.HandleError(err, resource, "operation name")
    }
    
    // Handle secret-related errors
    secret, err := r.getSecret(ctx, secretName)
    if err != nil {
        return handler.HandleSecretError(err, resource, secretName, "fetch")
    }
    
    return nil
}
```

## Features

- **Automatic Sanitization**: Detects and redacts passwords, tokens, API keys, private keys, and other sensitive data
- **Error Categories**: Classify errors for better handling (authentication, authorization, configuration, etc.)
- **Secure Logging**: Logs safe messages at error level, internal details at debug level
- **Reconciler Integration**: Purpose-built handlers for Kubernetes reconcilers
- **Event Safety**: Kubernetes events automatically use sanitized messages
- **Error Wrapping**: Preserves error chains for `errors.Is` and `errors.As` compatibility

## Core Types

### SecureError

Wraps errors with sanitized user messages while preserving internal details:

```go
secErr := secerrors.NewSecureError(
    secerrors.ErrorCategoryAuthentication,
    "authentication failed",  // Safe user message
    originalError,            // Internal error (for debugging)
)
```

### ReconcilerErrorHandler

Centralized error handling for reconcilers:

```go
handler := secerrors.NewReconcilerErrorHandler(ctx, recorder)
```

## Key Functions

| Function | Purpose | Use Case |
|----------|---------|----------|
| `NewSecureError` | Create a secure error | Manual error creation |
| `Wrap` | Wrap error with safe message | Error wrapping |
| `WrapWithSanitization` | Auto-sanitize and wrap | Quick sanitization |
| `SafeErrorMessage` | Get safe error message | Ad-hoc sanitization |
| `SanitizeErrorMessage` | Sanitize any string | Custom sanitization |
| `LogError` | Log error securely | Error logging |
| `NewReconcilerErrorHandler` | Create reconciler handler | Reconciler setup |

## Sanitization Patterns

Automatically detects and redacts:

- Passwords: `password=value`, `pwd:value`, `passwd=value`
- API Keys: `api_key=value`, `apikey:value`
- Tokens: `token=value`, JWT tokens, auth tokens
- Secrets: `secret=value`, `secret_key=value`
- Private Keys: PEM-formatted keys
- AWS Keys: AKIA..., ASIA..., etc.
- Credentials in URLs: `user:password@host`
- Base64 secrets: Long base64 strings with sensitive keywords

All matched patterns are replaced with `[REDACTED]`.

## Error Categories

```go
const (
    ErrorCategoryAuthentication  // Auth failures
    ErrorCategoryAuthorization   // Permission issues
    ErrorCategoryConfiguration   // Config errors
    ErrorCategoryValidation      // Validation failures
    ErrorCategoryInternal        // Internal errors
    ErrorCategoryNetwork         // Network errors
    ErrorCategoryStorage         // Storage errors
)
```

## Examples

### Basic Error Wrapping

```go
err := database.Connect()
if err != nil {
    return secerrors.Wrap(err, 
        secerrors.ErrorCategoryStorage, 
        "database connection failed")
}
```

### Automatic Sanitization

```go
// Error contains: "auth failed: password=secret123"
secErr := secerrors.WrapWithSanitization(err, secerrors.ErrorCategoryAuthentication)
// secErr.Error() returns: "auth failed: [REDACTED]"
```

### Secure Logging

```go
logger := logging.FromContext(ctx)
secerrors.LogError(logger, err, "operation failed", 
    "resource", resourceName)
// Logs safe message at ERROR level
// Logs internal details at DEBUG level
```

### Reconciler Error Handling

```go
handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)

// General errors
if err := r.reconcile(); err != nil {
    return handler.HandleError(err, resource, "reconciliation")
}

// Secret errors
if err := r.validateSecret(); err != nil {
    return handler.HandleSecretError(err, resource, secretName, "validation")
}

// Auth errors
if err := r.authenticate(); err != nil {
    return handler.HandleAuthError(err, resource, "authentication")
}

// Validation errors
if err := r.validate(); err != nil {
    return handler.HandleValidationError(err, resource, fieldName)
}
```

### Status Conditions

```go
if err != nil {
    // Sanitize before adding to status
    safeMsg := secerrors.GetSanitizedConditionMessage(err, "operation failed")
    resource.Status.MarkFalse(ConditionReady, "OperationFailed", safeMsg)
}
```

## Testing

```go
func TestErrorSanitization(t *testing.T) {
    err := errors.New("auth failed: password=secret123")
    safeMsg := secerrors.SafeErrorMessage(err)
    
    assert.NotContains(t, safeMsg, "secret123")
    assert.Contains(t, safeMsg, "[REDACTED]")
}
```

## Pre-defined Errors

Common errors available for reuse:

```go
secerrors.ErrAuthenticationFailed  // Generic auth failure
secerrors.ErrAuthorizationFailed   // Permission denied
secerrors.ErrInvalidConfiguration  // Config validation failed
secerrors.ErrSecretNotFound        // Secret not found
secerrors.ErrInvalidSecret         // Secret format invalid
```

## Best Practices

✅ **DO:**
- Use `ReconcilerErrorHandler` for consistency
- Log errors with `secerrors.LogError()`
- Wrap errors with appropriate categories
- Use sanitized messages in status conditions
- Check error categories with `IsCategory()`

❌ **DON'T:**
- Use `fmt.Errorf` with `%v` for sensitive errors
- Log raw errors that might contain secrets
- Include actual secret values in error messages
- Differentiate error messages for user enumeration
- Expose internal implementation details

## Documentation

- [Secure Error Handling Guide](../../../docs/SecureErrorHandling.md)
- [Code Examples](../../../docs/SecureErrorHandlingExamples.md)
- [Reconciler Update Example](../../../docs/SecureErrorHandling_ReconcilerUpdateExample.md)

## Related Standards

- [OWASP Error Handling](https://cheatsheetseries.owasp.org/cheatsheets/Error_Handling_Cheat_Sheet.html)
- [CWE-209: Information Exposure Through an Error Message](https://cwe.mitre.org/data/definitions/209.html)
- [CWE-532: Information Exposure Through Log Files](https://cwe.mitre.org/data/definitions/532.html)

## License

Apache License 2.0 - see [LICENSE](../../../LICENSE) for details.

