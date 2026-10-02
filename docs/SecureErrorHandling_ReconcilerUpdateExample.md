# Practical Example: Updating TektonHub Reconciler with Secure Error Handling

This document shows a complete before/after example of updating the TektonHub reconciler to use secure error handling.

## File: `pkg/reconciler/kubernetes/tektonhub/tektonhub.go`

### Changes Required

#### 1. Add Import

```go
import (
    // ... existing imports ...
    "github.com/tektoncd/operator/pkg/common/secerrors"
)
```

#### 2. Update FinalizeKind Method

**Before:**
```go
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonHub) pkgreconciler.Event {
    logger := logging.FromContext(ctx)

    labelSelector, err := common.LabelSelector(ls)
    if err != nil {
        return err
    }
    if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
        DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
            LabelSelector: labelSelector,
        }); err != nil {
        logger.Error("Failed to delete installer set created by TektonHub", err)
        return err
    }

    if err := r.extension.Finalize(ctx, original); err != nil {
        logger.Error("Failed to finalize platform resources", err)
    }
    return nil
}
```

**After:**
```go
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonHub) pkgreconciler.Event {
    handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)

    labelSelector, err := common.LabelSelector(ls)
    if err != nil {
        return handler.HandleError(err, original, "label selector creation")
    }
    
    if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
        DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
            LabelSelector: labelSelector,
        }); err != nil {
        return handler.HandleError(err, original, "installer set cleanup")
    }

    if err := r.extension.Finalize(ctx, original); err != nil {
        return handler.HandleError(err, original, "platform resource finalization")
    }
    return nil
}
```

#### 3. Update ReconcileKind for Database Secret Handling

**Before:**
```go
func (r *Reconciler) ensureDatabaseSecret(ctx context.Context, th *v1alpha1.TektonHub) error {
    logger := logging.FromContext(ctx)
    
    dbSpec := th.Spec.Db
    if dbSpec.DbSecretName != "" {
        // external DB
        secret, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Get(ctx, dbSpec.DbSecretName, metav1.GetOptions{})
        if err != nil {
            logger.Error("error getting db secret: ", err)
            return err
        }
        
        for _, key := range dbKeys {
            if _, ok := secret.Data[key]; !ok {
                return errKeyMissing
            }
        }
        return nil
    }
    
    // internal DB - create secret
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      databaseSecretName,
            Namespace: th.Spec.GetTargetNamespace(),
        },
        Type: corev1.SecretTypeOpaque,
        StringData: map[string]string{
            secretKeyPostgresHost:     defaultPostgresHost,
            secretKeyPostgresDB:       defaultPostgresDB,
            secretKeyPostgresUser:     defaultPostgresUser,
            secretKeyPostgresPassword: defaultPostgresPassword,
            secretKeyPostgresPort:     defaultPostgresPort,
        },
    }
    
    _, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Create(ctx, secret, metav1.CreateOptions{})
    if err != nil && !apierrors.IsAlreadyExists(err) {
        logger.Error("error creating db secret: ", err)
        return err
    }
    
    return nil
}
```

**After:**
```go
func (r *Reconciler) ensureDatabaseSecret(ctx context.Context, th *v1alpha1.TektonHub) error {
    handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)
    
    dbSpec := th.Spec.Db
    if dbSpec.DbSecretName != "" {
        // external DB - validate provided secret
        secret, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Get(
            ctx, dbSpec.DbSecretName, metav1.GetOptions{})
        if err != nil {
            return handler.HandleSecretError(err, th, dbSpec.DbSecretName, "fetch")
        }
        
        // Validate all required keys are present
        for _, key := range dbKeys {
            if _, ok := secret.Data[key]; !ok {
                err := fmt.Errorf("missing required key: %s", key)
                return handler.HandleValidationError(err, th, "database secret")
            }
        }
        return nil
    }
    
    // internal DB - create default secret
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      databaseSecretName,
            Namespace: th.Spec.GetTargetNamespace(),
        },
        Type: corev1.SecretTypeOpaque,
        StringData: map[string]string{
            secretKeyPostgresHost:     defaultPostgresHost,
            secretKeyPostgresDB:       defaultPostgresDB,
            secretKeyPostgresUser:     defaultPostgresUser,
            secretKeyPostgresPassword: defaultPostgresPassword,
            secretKeyPostgresPort:     defaultPostgresPort,
        },
    }
    
    _, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Create(
        ctx, secret, metav1.CreateOptions{})
    if err != nil && !apierrors.IsAlreadyExists(err) {
        return handler.HandleSecretError(err, th, databaseSecretName, "create")
    }
    
    return nil
}
```

#### 4. Update Error Handling in ReconcileKind

**Before:**
```go
func (r *Reconciler) ReconcileKind(ctx context.Context, th *v1alpha1.TektonHub) pkgreconciler.Event {
    logger := logging.FromContext(ctx)
    th.Status.InitializeConditions()
    th.Status.ObservedGeneration = th.Generation

    logger.Infow("Reconciling TektonHub", "status", th.Status)

    // validation
    if th.GetName() != v1alpha1.HubResourceName {
        msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
            v1alpha1.HubResourceName,
            th.GetName(),
        )
        logger.Error(msg)
        th.Status.MarkNotReady(msg)
        return nil
    }

    th.SetDefaults(ctx)

    // reconcile target namespace
    if err := common.ReconcileTargetNamespace(ctx, nil, nil, th, r.kubeClientSet); err != nil {
        logger.Errorw("error on reconciling targetNamespace",
            "targetNamespace", th.Spec.GetTargetNamespace(),
            err,
        )
        return err
    }

    // ensure database secret exists
    if err := r.ensureDatabaseSecret(ctx, th); err != nil {
        logger.Error("error ensuring database secret: ", err)
        th.Status.MarkNotReady("database configuration failed")
        return err
    }

    // ... rest of reconciliation ...
}
```

**After:**
```go
func (r *Reconciler) ReconcileKind(ctx context.Context, th *v1alpha1.TektonHub) pkgreconciler.Event {
    logger := logging.FromContext(ctx)
    handler := secerrors.NewReconcilerErrorHandler(ctx, r.Recorder)
    
    th.Status.InitializeConditions()
    th.Status.ObservedGeneration = th.Generation

    logger.Infow("Reconciling TektonHub", "status", th.Status)

    // validation
    if th.GetName() != v1alpha1.HubResourceName {
        msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
            v1alpha1.HubResourceName,
            th.GetName(),
        )
        logger.Warn(msg)
        th.Status.MarkNotReady(msg)
        return nil
    }

    th.SetDefaults(ctx)

    // reconcile target namespace
    if err := common.ReconcileTargetNamespace(ctx, nil, nil, th, r.kubeClientSet); err != nil {
        secerrors.LogError(logger, err, "target namespace reconciliation failed",
            "targetNamespace", th.Spec.GetTargetNamespace())
        return handler.HandleError(err, th, "target namespace reconciliation")
    }

    // ensure database secret exists
    if err := r.ensureDatabaseSecret(ctx, th); err != nil {
        // Error already handled by ensureDatabaseSecret with proper sanitization
        safeMsg := secerrors.GetSanitizedConditionMessage(err, "database configuration failed")
        th.Status.MarkNotReady(safeMsg)
        return err
    }

    // ... rest of reconciliation ...
}
```

## Summary of Changes

### Security Improvements

1. **Secret Operations**: All secret-related operations now use `HandleSecretError` which:
   - Prevents secret names/values from appearing in logs
   - Creates generic, safe error messages for users
   - Preserves internal details for debugging

2. **Error Categorization**: Errors are now categorized:
   - Configuration errors (secrets, config)
   - Validation errors (missing keys, invalid format)
   - Internal errors (unexpected failures)

3. **Consistent Logging**: All error logging uses secure functions:
   - `secerrors.LogError()` - Safe message at error level, internal at debug
   - `handler.HandleError()` - Records sanitized Kubernetes events

4. **Status Conditions**: Status messages use sanitized errors:
   - `GetSanitizedConditionMessage()` - Removes sensitive data from condition messages

### Benefits

- ✅ No database credentials exposed in logs
- ✅ Secret names and values sanitized
- ✅ Consistent error handling across the reconciler
- ✅ Better user experience with clear, safe error messages
- ✅ Debugging information preserved (at debug log level)
- ✅ Kubernetes events don't leak sensitive information

## Testing the Changes

### Unit Test Example

```go
func TestEnsureDatabaseSecret_SecureErrors(t *testing.T) {
    ctx := context.Background()
    recorder := &fakeRecorder{}
    
    // Test missing secret
    reconciler := &Reconciler{
        kubeClientSet: fakeClient,
        Recorder: recorder,
    }
    
    th := &v1alpha1.TektonHub{
        Spec: v1alpha1.TektonHubSpec{
            Db: v1alpha1.DbSpec{
                DbSecretName: "nonexistent-secret",
            },
        },
    }
    
    err := reconciler.ensureDatabaseSecret(ctx, th)
    
    // Verify error is secure
    assert.NotNil(t, err)
    assert.NotContains(t, err.Error(), "nonexistent-secret") // Secret name should be sanitized
    assert.True(t, secerrors.IsCategory(err, secerrors.ErrorCategoryConfiguration))
    
    // Verify event was recorded with safe message
    events := recorder.GetEvents()
    assert.Len(t, events, 1)
    assert.NotContains(t, events[0].Message, "password")
    assert.NotContains(t, events[0].Message, "token")
}
```

### Manual Testing

1. **Create TektonHub with missing secret:**
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonHub
metadata:
  name: hub
spec:
  targetNamespace: tekton-hub
  db:
    dbSecretName: nonexistent-secret
```

2. **Check logs** - should show sanitized messages:
```
ERROR: secret operation failed for fetch
DEBUG: internal error: secrets "nonexistent-secret" not found
```

3. **Check events** - should show safe messages:
```bash
kubectl get events -n tekton-hub
# LAST SEEN   TYPE      REASON                  MESSAGE
# 1m          Warning   SecretOperationFailed   secret operation failed for fetch
```

4. **Check status conditions** - should not expose secrets:
```bash
kubectl get tektonhub hub -o yaml
# status:
#   conditions:
#   - message: database configuration failed: secret operation failed
#     reason: NotReady
#     status: "False"
```

## Rollout Strategy

1. **Phase 1**: Update core reconcilers (TektonHub, TektonResult, TektonChain) that handle secrets
2. **Phase 2**: Update remaining reconcilers for consistency
3. **Phase 3**: Add linter rules to enforce secure error handling
4. **Phase 4**: Update documentation and provide team training

## Next Steps

1. Apply these changes to `pkg/reconciler/kubernetes/tektonhub/tektonhub.go`
2. Run tests: `go test ./pkg/reconciler/kubernetes/tektonhub/...`
3. Verify no sensitive data in logs with integration tests
4. Repeat pattern for other reconcilers

