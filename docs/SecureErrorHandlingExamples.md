# Secure Error Handling - Code Examples

This document provides concrete examples of implementing secure error handling in Tekton Operator reconcilers.

## Example 1: TektonHub Database Secret Handling

### Before (Insecure)

```go
func (r *Reconciler) ensureDatabaseSecret(ctx context.Context, th *v1alpha1.TektonHub) error {
    logger := logging.FromContext(ctx)
    
    secret, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.TargetNamespace).Get(
        ctx, databaseSecretName, metav1.GetOptions{})
    if err != nil {
        logger.Error("failed to get database secret:", err)  // May expose secret details
        return fmt.Errorf("secret operation failed: %v", err)  // May leak info
    }
    
    // Validate secret has all required keys
    for _, key := range dbKeys {
        if _, ok := secret.Data[key]; !ok {
            return fmt.Errorf("secret %s missing key: %s", databaseSecretName, key)  // Exposes secret structure
        }
    }
    
    return nil
}
```

### After (Secure)

```go
func (r *Reconciler) ensureDatabaseSecret(ctx context.Context, th *v1alpha1.TektonHub) error {
    handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)
    
    secret, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.TargetNamespace).Get(
        ctx, databaseSecretName, metav1.GetOptions{})
    if err != nil {
        return handler.HandleSecretError(err, th, databaseSecretName, "fetch")
    }
    
    // Validate secret has all required keys
    for _, key := range dbKeys {
        if _, ok := secret.Data[key]; !ok {
            err := fmt.Errorf("missing required key: %s", key)
            return handler.HandleValidationError(err, th, "database secret")
        }
    }
    
    return nil
}
```

**Benefits:**
- Secret details not exposed in logs
- Consistent error messages and events
- Internal error preserved for debugging
- Kubernetes events automatically sanitized

## Example 2: Cosign Key Generation

### Before (Insecure)

```go
func generateSigningSecrets(ctx context.Context) map[string][]byte {
    logger := logging.FromContext(ctx)
    
    randomPassword, err := generateRandomPassword(ctx)
    if err != nil {
        logger.Error("Error generating random password %w:", err)  // Generic but inconsistent
        return nil
    }
    
    passFunc := func(confirm bool) ([]byte, error) {
        return []byte(randomPassword), nil
    }
    
    keys, err := cosign.GenerateKeyPair(passFunc)
    if err != nil {
        logger.Error("Error generating cosign key pair:", err)  // May expose crypto details
        return nil
    }
    
    return map[string][]byte{
        "cosign.key":      keys.PrivateBytes,  // Returning private key in map (OK in this context)
        "cosign.pub":      keys.PublicBytes,
        "cosign.password": []byte(randomPassword),
    }
}
```

### After (Secure)

```go
func generateSigningSecrets(ctx context.Context) (map[string][]byte, error) {
    logger := logging.FromContext(ctx)
    
    randomPassword, err := generateRandomPassword(ctx)
    if err != nil {
        secErr := secerrors.Wrap(err, secerrors.ErrorCategoryInternal, 
            "failed to generate secure password")
        secerrors.LogError(logger, secErr, "password generation failed")
        return nil, secErr
    }
    
    passFunc := func(confirm bool) ([]byte, error) {
        return []byte(randomPassword), nil
    }
    
    keys, err := cosign.GenerateKeyPair(passFunc)
    if err != nil {
        secErr := secerrors.Wrap(err, secerrors.ErrorCategoryInternal, 
            "failed to generate signing key pair")
        secerrors.LogError(logger, secErr, "key pair generation failed")
        return nil, secErr
    }
    
    return map[string][]byte{
        "cosign.key":      keys.PrivateBytes,
        "cosign.pub":      keys.PublicBytes,
        "cosign.password": []byte(randomPassword),
    }, nil
}
```

**Benefits:**
- Cryptographic errors don't expose implementation details
- Consistent error categorization
- Returns error for proper handling by caller
- Secure logging with debug-level internal details

## Example 3: Webhook Mutation Error

### Before (Insecure)

```go
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
    logger := logging.FromContext(ctx)
    
    patchBytes, err := ac.mutate(ctx, request)
    if err != nil {
        return webhook.MakeErrorStatus("mutation failed: %v", err)  // Exposes internal error
    }
    logger.Infof("Kind: %q PatchBytes: %v", request.Kind, string(patchBytes))  // May log sensitive data
    
    return &admissionv1.AdmissionResponse{
        Patch:   patchBytes,
        Allowed: true,
        PatchType: func() *admissionv1.PatchType {
            pt := admissionv1.PatchTypeJSONPatch
            return &pt
        }(),
    }
}
```

### After (Secure)

```go
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
    logger := logging.FromContext(ctx)
    
    patchBytes, err := ac.mutate(ctx, request)
    if err != nil {
        safeMsg := secerrors.SafeErrorMessage(err)
        secerrors.LogError(logger, err, "mutation failed", 
            "kind", request.Kind.Kind, 
            "namespace", request.Namespace)
        return webhook.MakeErrorStatus("mutation failed: %s", safeMsg)
    }
    
    // Only log patch size, not content (which might contain secrets)
    logger.Infof("Kind: %q PatchSize: %d bytes", request.Kind, len(patchBytes))
    
    return &admissionv1.AdmissionResponse{
        Patch:   patchBytes,
        Allowed: true,
        PatchType: func() *admissionv1.PatchType {
            pt := admissionv1.PatchTypeJSONPatch
            return &pt
        }(),
    }
}
```

**Benefits:**
- Webhook errors sanitized before returning to client
- Patch content not logged (may contain secrets/sensitive data)
- Structured logging with safe context

## Example 4: Database Connection Error

### Before (Insecure)

```go
func (r *Reconciler) connectDatabase(props v1alpha1.ResultsAPIProperties) error {
    connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
        props.DBUser, props.DBPassword, props.DBHost, props.DBPort, props.DBName)
    
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return fmt.Errorf("failed to connect to database: %v", err)  // May include connection string with password
    }
    
    if err := db.Ping(); err != nil {
        return fmt.Errorf("database ping failed: %v", err)  // May expose DB details
    }
    
    return nil
}
```

### After (Secure)

```go
func (r *Reconciler) connectDatabase(props v1alpha1.ResultsAPIProperties) error {
    // Build connection string (never log this)
    connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
        props.DBUser, props.DBPassword, props.DBHost, props.DBPort, props.DBName)
    
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        // Don't include the original error (contains connection string)
        return secerrors.NewSecureError(
            secerrors.ErrorCategoryStorage,
            "failed to connect to database",
            err,  // Preserved internally for debugging
        )
    }
    
    if err := db.Ping(); err != nil {
        return secerrors.Wrap(err, secerrors.ErrorCategoryNetwork, 
            "database health check failed")
    }
    
    return nil
}
```

**Benefits:**
- Connection strings with passwords never exposed
- Database internal errors sanitized
- Original error preserved for secure debugging
- Clear error categorization

## Example 5: Configuration Validation

### Before (Insecure)

```go
func validateConfig(config *Config) error {
    if config.APIToken == "" {
        return errors.New("API token is required")  // OK, doesn't expose token
    }
    
    if !strings.HasPrefix(config.APIToken, "tkn_") {
        return fmt.Errorf("invalid API token format: %s", config.APIToken)  // EXPOSES TOKEN!
    }
    
    if config.SecretKey == "" {
        return errors.New("secret key is required")
    }
    
    return nil
}
```

### After (Secure)

```go
func validateConfig(config *Config) error {
    if config.APIToken == "" {
        return secerrors.NewSecureError(
            secerrors.ErrorCategoryValidation,
            "API token is required",
            errors.New("missing API token"),
        )
    }
    
    if !strings.HasPrefix(config.APIToken, "tkn_") {
        // Don't include the actual token in error message
        return secerrors.NewSecureError(
            secerrors.ErrorCategoryValidation,
            "invalid API token format (expected prefix: tkn_)",
            fmt.Errorf("invalid token prefix"), // No actual token in internal error either
        )
    }
    
    if config.SecretKey == "" {
        return secerrors.NewSecureError(
            secerrors.ErrorCategoryValidation,
            "secret key is required",
            errors.New("missing secret key"),
        )
    }
    
    return nil
}
```

**Benefits:**
- Never exposes actual token/key values
- Provides helpful validation messages without revealing secrets
- Consistent error categorization

## Example 6: Error in Status Conditions

### Before (Insecure)

```go
func (r *Reconciler) updateStatus(ctx context.Context, tr *v1alpha1.TektonResult) error {
    err := r.performOperation()
    if err != nil {
        tr.Status.MarkInstallFailed(fmt.Sprintf("installation failed: %v", err))  // May expose secrets
    }
    return err
}
```

### After (Secure)

```go
func (r *Reconciler) updateStatus(ctx context.Context, tr *v1alpha1.TektonResult) error {
    err := r.performOperation()
    if err != nil {
        // Use sanitized message in status condition
        safeMsg := secerrors.GetSanitizedConditionMessage(err, "installation failed")
        tr.Status.MarkInstallFailed(safeMsg)
    }
    return err
}
```

**Benefits:**
- Status conditions visible to users don't expose secrets
- Error details still available for debugging via logs
- Consistent user-facing messages

## Pattern Summary

| Scenario | Use This Function | Example |
|----------|-------------------|---------|
| General reconciler errors | `ReconcilerErrorHandler.HandleError` | Operation failures |
| Secret operations | `ReconcilerErrorHandler.HandleSecretError` | Secret fetch/validation |
| Authentication | `ReconcilerErrorHandler.HandleAuthError` | Login failures |
| Validation | `ReconcilerErrorHandler.HandleValidationError` | Input validation |
| Configuration | `ReconcilerErrorHandler.HandleConfigError` | Config validation |
| Logging errors | `secerrors.LogError` | Any error logging |
| Status conditions | `secerrors.GetSanitizedConditionMessage` | Setting resource status |
| Quick sanitization | `secerrors.SafeErrorMessage` | Ad-hoc sanitization |
| Wrapping errors | `secerrors.Wrap` or `secerrors.WrapWithSanitization` | Error wrapping |

## Testing Secure Errors

```go
func TestSecureErrorHandling(t *testing.T) {
    // Test that sensitive data is redacted
    err := errors.New("auth failed with password: secret123")
    handler := secerrors.NewReconcilerErrorHandler(context.Background(), nil)
    
    secErr := handler.HandleError(err, nil, "authentication")
    
    // Verify sanitization
    if strings.Contains(secErr.Error(), "secret123") {
        t.Error("password not sanitized in error message")
    }
    
    // Verify error is categorized
    if !secerrors.IsCategory(secErr, secerrors.ErrorCategoryInternal) {
        t.Error("error not properly categorized")
    }
}
```

## Checklist for Code Reviews

When reviewing code, ensure:

- [ ] No `fmt.Errorf` with `%v` that could expose secrets
- [ ] All errors from secret operations are sanitized
- [ ] Database connection strings are never logged
- [ ] Authentication errors don't differentiate between "user not found" and "wrong password"
- [ ] Validation errors don't include the actual invalid value if it could be sensitive
- [ ] Status conditions use sanitized messages
- [ ] Webhook errors are sanitized before returning to clients
- [ ] All error logging uses `secerrors.LogError` or equivalent
- [ ] Test cases verify sanitization works correctly

