# Manual Test Plan: Centralized TLS Configuration

## Overview
This test plan validates the centralized TLS configuration feature for Tekton components on OpenShift. All tests use configuration changes only - no operator redeployment required.

## Prerequisites

- OpenShift cluster with Tekton Operator installed
- `oc` CLI configured and authenticated
- TektonResult component installed
- Access to modify cluster-scoped resources (APIServer)

## Test Environment Setup

```bash
# Set namespace variable
export TEKTON_NAMESPACE=openshift-pipelines

# Verify TektonConfig exists
oc get tektonconfig config -o yaml

# Verify APIServer resource exists
oc get apiserver cluster -o yaml

# Verify TektonResult is installed
oc get tektonresult -n $TEKTON_NAMESPACE

# Get TektonResult deployment (used throughout tests)
export RESULT_DEPLOY=$(oc get deployment -n $TEKTON_NAMESPACE -l app.kubernetes.io/part-of=tekton-results -o name | head -1)
echo "TektonResult deployment: $RESULT_DEPLOY"
```

---

## Understanding Installer Set Updates

The installer set is **updated in-place** (same name), not recreated. When TLS config changes:
1. TektonResult CR gets `operator.tekton.dev/platform-data-hash` annotation
2. This annotation is included in hash computation
3. Installer set's `operator.tekton.dev/last-applied-hash` annotation is updated
4. Installer set manifests are updated
5. Deployment rolls out

**What to check**: Installer set's `last-applied-hash` annotation changes (NOT the installer set name).

---

## Test Suite

### Test 1: Baseline - Feature Disabled (Default State)

**Objective**: Verify default behavior when centralized TLS config is disabled.

**Steps**:
```bash
# 1. Verify EnableCentralTLSConfig is false (default)
FEATURE_ENABLED=$(oc get tektonconfig config -o jsonpath='{.spec.platforms.openshift.enableCentralTLSConfig}')
echo "Feature enabled: ${FEATURE_ENABLED:-false}"
# Expected: false or empty

# 2. Check for TLS env vars (should NOT exist)
TLS_MIN=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].name}')
TLS_CIPHER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].name}')

if [ -z "$TLS_MIN" ] && [ -z "$TLS_CIPHER" ]; then
  echo "✓ No TLS env vars present (expected)"
else
  echo "✗ Unexpected TLS env vars found"
fi

# 3. Record baseline state
INSTALLERSET_NAME=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.name}')
HASH_ANNOTATION=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')

echo "Installer set name: $INSTALLERSET_NAME"
echo "Installer set hash annotation: $HASH_ANNOTATION"

# 4. Check TektonResult platform-data-hash (should not exist)
PLATFORM_HASH=$(oc get tektonresult result -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.operator\.tekton\.dev/platform-data-hash}')
if [ -z "$PLATFORM_HASH" ]; then
  echo "✓ No platform-data-hash annotation (expected)"
else
  echo "⚠ Unexpected platform-data-hash: $PLATFORM_HASH"
fi

# 5. Check current APIServer TLS profile
APISERVER_PROFILE=$(oc get apiserver cluster -o jsonpath='{.spec.tlsSecurityProfile.type}')
if [ -z "$APISERVER_PROFILE" ]; then
  echo "APIServer has no TLS profile (null)"
else
  echo "APIServer profile: $APISERVER_PROFILE"
fi
```

**Expected Results**:
- `enableCentralTLSConfig` is `false` or not set
- TLS env vars NOT present in deployment
- No `platform-data-hash` annotation on TektonResult
- Baseline installer set state recorded

---

### Test 2: Enable Feature with Library-Go Defaults

**Objective**: Verify that enabling the feature (even without explicit APIServer profile) injects library-go default TLS values.

**Steps**:
```bash
# 1. Ensure APIServer has no explicit TLS profile
APISERVER_PROFILE=$(oc get apiserver cluster -o jsonpath='{.spec.tlsSecurityProfile.type}')
if [ -n "$APISERVER_PROFILE" ]; then
  echo "Removing existing APIServer TLS profile to test defaults..."
  oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":null}}'
  sleep 35
fi

# 2. Record current state
INSTALLERSET_NAME=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.name}')
HASH_BEFORE=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
REV_BEFORE=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

echo "Installer set name: $INSTALLERSET_NAME"
echo "Hash annotation before: $HASH_BEFORE"
echo "Deployment revision before: $REV_BEFORE"

# 3. Enable centralized TLS config
oc patch tektonconfig config --type=merge -p '{"spec":{"platforms":{"openshift":{"enableCentralTLSConfig":true}}}}'

# 4. Wait for reconciliation
sleep 35

# 5. Verify installer set name unchanged, but hash annotation updated
INSTALLERSET_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.name}')
HASH_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')

echo "Installer set name after: $INSTALLERSET_AFTER"
echo "Hash annotation after: $HASH_AFTER"

if [ "$INSTALLERSET_NAME" = "$INSTALLERSET_AFTER" ]; then
  echo "✓ Installer set name unchanged (expected - updated in-place)"
else
  echo "✗ Installer set name changed unexpectedly"
fi

if [ "$HASH_BEFORE" != "$HASH_AFTER" ]; then
  echo "✓ Installer set hash annotation changed (library-go defaults now in hash)"
else
  echo "✗ Hash annotation unchanged"
fi

# 6. Verify TektonResult platform-data-hash annotation
PLATFORM_HASH=$(oc get tektonresult result -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.operator\.tekton\.dev/platform-data-hash}')
echo "TektonResult platform-data-hash: $PLATFORM_HASH"

if [ -n "$PLATFORM_HASH" ]; then
  echo "✓ Platform data hash annotation set on TektonResult"
else
  echo "✗ Platform data hash annotation missing"
fi

# 7. Wait for deployment rollout
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 8. Verify deployment revision incremented
REV_AFTER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

if [ "$REV_AFTER" -gt "$REV_BEFORE" ]; then
  echo "✓ Deployment rolled out (revision $REV_BEFORE → $REV_AFTER)"
fi

# 9. Verify TLS env vars injected with library-go defaults
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')

echo "TLS_MIN_VERSION: $MIN_VERSION"
echo "TLS_CIPHER_SUITES: $CIPHER_SUITES"

# 10. Verify library-go default values
if [ "$MIN_VERSION" = "1.2" ]; then
  echo "✓ TLS version set to 1.2 (library-go default)"
else
  echo "✗ Expected TLS 1.2, got: $MIN_VERSION"
fi

# Expected default ciphers (6 TLS 1.2 ciphers)
EXPECTED_CIPHERS="TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"

if [ "$CIPHER_SUITES" = "$EXPECTED_CIPHERS" ]; then
  echo "✓ Cipher suites match library-go defaults (6 TLS 1.2 ciphers)"
else
  echo "⚠ Cipher suites differ from expected defaults"
fi

# 11. Note: library-go defaults do NOT include TLS 1.3 ciphers
echo "$CIPHER_SUITES" | grep -q "TLS_AES"
if [ $? -ne 0 ]; then
  echo "✓ No TLS 1.3 ciphers in library-go defaults (expected)"
else
  echo "⚠ Unexpected TLS 1.3 ciphers found in defaults"
fi
```

**Expected Results**:
- Installer set name unchanged (updated in-place)
- Installer set `last-applied-hash` annotation changed
- TektonResult `platform-data-hash` annotation set
- Deployment rolled out
- `TLS_MIN_VERSION` = "1.2" (library-go default)
- `TLS_CIPHER_SUITES` = 6 TLS 1.2 ciphers (library-go defaults)
- No TLS 1.3 ciphers in defaults

**Note**: This behavior is by design - library-go provides safe defaults when no explicit profile is configured.

---

### Test 3: Set APIServer Profile to Intermediate

**Objective**: Verify explicit APIServer profile overrides library-go defaults.

**Steps**:
```bash
# 1. Record current state
HASH_BEFORE=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
PLATFORM_HASH_BEFORE=$(oc get tektonresult result -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.operator\.tekton\.dev/platform-data-hash}')
REV_BEFORE=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

echo "Hash annotation before: $HASH_BEFORE"
echo "Platform hash before: $PLATFORM_HASH_BEFORE"
echo "Revision before: $REV_BEFORE"

# 2. Set APIServer to Intermediate profile
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Intermediate","intermediate":{}}}}'

# 3. Wait for APIServer watch to trigger reconciliation
echo "Waiting for reconciliation..."
sleep 35

# 4. Verify hash annotations changed
HASH_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
PLATFORM_HASH_AFTER=$(oc get tektonresult result -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.operator\.tekton\.dev/platform-data-hash}')

echo "Hash annotation after: $HASH_AFTER"
echo "Platform hash after: $PLATFORM_HASH_AFTER"

if [ "$HASH_BEFORE" != "$HASH_AFTER" ]; then
  echo "✓ Installer set hash annotation changed (Intermediate profile differs from defaults)"
else
  echo "⚠ Hash annotation unchanged"
fi

if [ "$PLATFORM_HASH_BEFORE" != "$PLATFORM_HASH_AFTER" ]; then
  echo "✓ Platform data hash changed"
fi

# 5. Wait for deployment rollout
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 6. Verify deployment revision
REV_AFTER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

if [ "$REV_AFTER" -gt "$REV_BEFORE" ]; then
  echo "✓ Deployment rolled out (revision $REV_BEFORE → $REV_AFTER)"
fi

# 7. Verify TLS env vars
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')

echo "TLS_MIN_VERSION: $MIN_VERSION"
echo "TLS_CIPHER_SUITES: $CIPHER_SUITES"

# 8. Verify Intermediate profile values
if [ "$MIN_VERSION" = "1.2" ]; then
  echo "✓ TLS version set to 1.2 (Intermediate profile)"
else
  echo "✗ Expected TLS 1.2, got: $MIN_VERSION"
fi

# 9. Verify TLS 1.3 ciphers supplemented (key difference from defaults)
echo "$CIPHER_SUITES" | grep -q "TLS_AES_128_GCM_SHA256"
if [ $? -eq 0 ]; then
  echo "✓ TLS 1.3 ciphers supplemented (differs from library-go defaults)"
else
  echo "✗ TLS 1.3 ciphers not supplemented"
fi

# 10. Check operator logs
oc logs -n openshift-operators deployment/openshift-pipelines-operator --tail=100 | grep -E "APIServer TLS|TektonResult spec changed" | tail -5
```

**Expected Results**:
- Installer set `last-applied-hash` annotation changed
- TektonResult `platform-data-hash` annotation changed
- Deployment rolled out
- `TLS_MIN_VERSION` = "1.2"
- `TLS_CIPHER_SUITES` contains Intermediate ciphers + TLS 1.3 ciphers
- Key difference: TLS 1.3 ciphers present (supplemented by code)

---

### Test 4: Change APIServer Profile to Modern

**Objective**: Verify that changing the cluster TLS profile triggers TektonResult update.

**Steps**:
```bash
# 1. Record current state
HASH_BEFORE=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
REV_BEFORE=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

echo "Hash before: $HASH_BEFORE"
echo "Revision before: $REV_BEFORE"

# 2. Change APIServer to Modern profile
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Modern","modern":{}}}}'

# 3. Wait for reconciliation
echo "Waiting for reconciliation..."
sleep 35

# 4. Verify hash annotation changed
HASH_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
echo "Hash after: $HASH_AFTER"

if [ "$HASH_BEFORE" != "$HASH_AFTER" ]; then
  echo "✓ Hash changed as expected"
else
  echo "✗ Hash did not change"
fi

# 5. Wait for deployment rollout
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 6. Verify deployment revision incremented
REV_AFTER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

if [ "$REV_AFTER" -gt "$REV_BEFORE" ]; then
  echo "✓ Deployment rolled out (revision $REV_BEFORE → $REV_AFTER)"
else
  echo "✗ Deployment not updated"
fi

# 7. Verify TLS env vars updated
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')

echo "TLS_MIN_VERSION: $MIN_VERSION"
echo "TLS_CIPHER_SUITES: $CIPHER_SUITES"

# 8. Verify Modern profile values (TLS 1.3 minimum)
if [ "$MIN_VERSION" = "1.3" ]; then
  echo "✓ TLS version updated to 1.3 (Modern profile)"
else
  echo "✗ Expected TLS 1.3, got: $MIN_VERSION"
fi

# 9. Verify only TLS 1.3 ciphers present
if ! echo "$CIPHER_SUITES" | grep -q "TLS_ECDHE"; then
  echo "✓ No TLS 1.2 ciphers (Modern profile uses only TLS 1.3)"
else
  echo "⚠ TLS 1.2 ciphers found in Modern profile"
fi
```

**Expected Results**:
- Installer set hash annotation changed
- Deployment rolled out
- `TLS_MIN_VERSION` = "1.3"
- `TLS_CIPHER_SUITES` contains only TLS 1.3 ciphers

---

### Test 5: Change APIServer Profile to Old

**Objective**: Verify transition to older profile works.

**Steps**:
```bash
# 1. Change to Old profile
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Old","old":{}}}}'

# 2. Wait for reconciliation
sleep 35

# 3. Wait for rollout
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 4. Verify TLS env vars
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')

echo "TLS_MIN_VERSION: $MIN_VERSION"
echo "TLS_CIPHER_SUITES: $CIPHER_SUITES"

# 5. Verify Old profile values (TLS 1.0 minimum)
if [ "$MIN_VERSION" = "1.0" ]; then
  echo "✓ TLS version set to 1.0 (Old profile)"
else
  echo "⚠ Expected TLS 1.0, got: $MIN_VERSION"
fi

# 6. Verify wide range of ciphers (Old profile is permissive)
CIPHER_COUNT=$(echo "$CIPHER_SUITES" | tr ',' '\n' | wc -l)
echo "Cipher count: $CIPHER_COUNT"

if [ "$CIPHER_COUNT" -gt 10 ]; then
  echo "✓ Old profile has many ciphers (permissive)"
fi
```

**Expected Results**:
- `TLS_MIN_VERSION` = "1.0"
- `TLS_CIPHER_SUITES` contains many ciphers (Old profile is most permissive)
- Includes TLS 1.3 ciphers (supplemented)

---

### Test 6: Custom TLS Profile

**Objective**: Verify custom TLS profile with specific ciphers.

**Steps**:
```bash
# 1. Apply custom profile with specific settings
# Note: Use OpenSSL-style cipher names for TLS 1.2 (not Go constants)
# HTTP/2 requires both ECDSA and RSA variants of AES_128_GCM_SHA256
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Custom","custom":{"ciphers":["ECDHE-ECDSA-AES128-GCM-SHA256","ECDHE-RSA-AES128-GCM-SHA256","ECDHE-ECDSA-AES256-GCM-SHA384","ECDHE-RSA-AES256-GCM-SHA384"],"minTLSVersion":"VersionTLS12"}}}}'

# 2. Wait for reconciliation
sleep 35

# 3. Wait for rollout
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 4. Verify exact cipher list
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')

echo "TLS_MIN_VERSION: $MIN_VERSION"
echo "TLS_CIPHER_SUITES: $CIPHER_SUITES"

# 5. Verify custom values
if [ "$MIN_VERSION" = "1.2" ]; then
  echo "✓ Custom min version (1.2) applied"
else
  echo "✗ Expected 1.2, got: $MIN_VERSION"
fi

echo "$CIPHER_SUITES" | grep -q "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
if [ $? -eq 0 ]; then
  echo "✓ Custom TLS 1.2 cipher suite present"
else
  echo "✗ Custom cipher suite missing"
fi

echo "$CIPHER_SUITES" | grep -q "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
if [ $? -eq 0 ]; then
  echo "✓ ECDSA variant present (HTTP/2 requirement)"
else
  echo "✗ ECDSA variant missing"
fi

# 6. Verify only TLS 1.2 ciphers (no TLS 1.3 ciphers in custom profile)
echo "$CIPHER_SUITES" | grep -q "TLS_AES"
if [ $? -ne 0 ]; then
  echo "✓ No TLS 1.3 ciphers (custom profile has only TLS 1.2)"
else
  echo "⚠ Unexpected TLS 1.3 ciphers found"
fi

# 7. Verify exact cipher count (4 TLS 1.2 ciphers from custom profile)
CIPHER_COUNT=$(echo "$CIPHER_SUITES" | tr ',' '\n' | wc -l)
echo "Cipher count: $CIPHER_COUNT (expected: 4)"
```

**Expected Results**:
- `TLS_MIN_VERSION` = "1.2"
- `TLS_CIPHER_SUITES` contains exactly the 4 specified TLS 1.2 ciphers
- NO TLS 1.3 ciphers (custom profiles don't support mixing TLS 1.2 and TLS 1.3 ciphers)
- Both ECDSA and RSA variants present (HTTP/2 requirement)

**Notes**:
- **TLS 1.2 cipher names**: Use OpenSSL format (e.g., `ECDHE-RSA-AES128-GCM-SHA256`)
- **TLS 1.3 ciphers**: Not configurable in custom profiles (use predefined profiles like Modern)
- The operator converts OpenSSL format to Go constants when setting environment variables
- Custom profiles can only configure TLS 1.2 ciphers with `minTLSVersion: VersionTLS12`
- For TLS 1.3, use the Modern predefined profile instead of custom configuration

---

### Test 7: Remove APIServer TLS Profile (Revert to Library-Go Defaults)

**Objective**: Verify that removing APIServer profile reverts to library-go defaults.

**Steps**:
```bash
# 1. Record state before removing profile
HASH_BEFORE=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
REV_BEFORE=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

# 2. Remove TLS profile (revert to library-go defaults)
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":null}}'

# 3. Wait for reconciliation
sleep 35

# 4. Check if hash annotation changed
HASH_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')

if [ "$HASH_BEFORE" != "$HASH_AFTER" ]; then
  echo "✓ Hash annotation changed (reverted to library-go defaults)"
else
  echo "⚠ Hash unchanged (previous profile may match defaults)"
fi

# 5. Wait for deployment
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 6. Check deployment revision
REV_AFTER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

if [ "$REV_AFTER" -gt "$REV_BEFORE" ]; then
  echo "✓ Deployment updated (revision $REV_BEFORE → $REV_AFTER)"
else
  echo "⚠ Deployment not updated (defaults may match previous profile)"
fi

# 7. Verify TLS env vars have library-go defaults
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')

echo "TLS_MIN_VERSION: $MIN_VERSION"
echo "TLS_CIPHER_SUITES: $CIPHER_SUITES"

# 8. Verify library-go defaults
if [ "$MIN_VERSION" = "1.2" ]; then
  echo "✓ Reverted to library-go default TLS 1.2"
else
  echo "⚠ Unexpected TLS version: $MIN_VERSION"
fi

EXPECTED_CIPHERS="TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"

if [ "$CIPHER_SUITES" = "$EXPECTED_CIPHERS" ]; then
  echo "✓ Reverted to library-go default ciphers (6 TLS 1.2 ciphers)"
else
  echo "⚠ Ciphers differ from library-go defaults"
fi

# 9. Verify no TLS 1.3 ciphers (defaults don't include them)
echo "$CIPHER_SUITES" | grep -q "TLS_AES"
if [ $? -ne 0 ]; then
  echo "✓ No TLS 1.3 ciphers (library-go defaults)"
else
  echo "⚠ Unexpected TLS 1.3 ciphers"
fi
```

**Expected Results**:
- Hash annotation changes if previous profile differed from defaults
- TLS env vars present with library-go defaults
- `TLS_MIN_VERSION` = "1.2"
- `TLS_CIPHER_SUITES` = 6 TLS 1.2 ciphers (no TLS 1.3)
- Same values as Test 2

**Note**: Removing the profile does NOT remove TLS env vars - it reverts to library-go defaults.

---

### Test 8: Disable Feature - Verify Cleanup

**Objective**: Disable centralized TLS config and verify TLS env vars are removed.

**Steps**:
```bash
# 1. First set a profile so we have explicit config
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Intermediate","intermediate":{}}}}'
sleep 35

# 2. Verify TLS env vars present
TLS_MIN=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
echo "Current TLS_MIN_VERSION: $TLS_MIN"

# 3. Record current state
HASH_BEFORE=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
PLATFORM_HASH_BEFORE=$(oc get tektonresult result -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.operator\.tekton\.dev/platform-data-hash}')
REV_BEFORE=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

echo "Hash before: $HASH_BEFORE"
echo "Platform hash before: $PLATFORM_HASH_BEFORE"

# 4. Disable centralized TLS config
oc patch tektonconfig config --type=merge -p '{"spec":{"platforms":{"openshift":{"enableCentralTLSConfig":false}}}}'

# 5. Wait for reconciliation
sleep 35

# 6. Verify hash annotations changed
HASH_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
PLATFORM_HASH_AFTER=$(oc get tektonresult result -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.operator\.tekton\.dev/platform-data-hash}')

echo "Hash after: $HASH_AFTER"
echo "Platform hash after: $PLATFORM_HASH_AFTER"

if [ "$HASH_BEFORE" != "$HASH_AFTER" ]; then
  echo "✓ Installer set hash annotation changed (TLS config removed)"
else
  echo "✗ Hash annotation unchanged"
fi

if [ -z "$PLATFORM_HASH_AFTER" ]; then
  echo "✓ Platform data hash annotation removed from TektonResult"
else
  echo "⚠ Platform data hash still present: $PLATFORM_HASH_AFTER"
fi

# 7. Wait for rollout
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 8. Verify deployment revision incremented
REV_AFTER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

if [ "$REV_AFTER" -gt "$REV_BEFORE" ]; then
  echo "✓ Deployment rolled out (revision $REV_BEFORE → $REV_AFTER)"
fi

# 9. Verify TLS env vars removed
TLS_MIN=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].name}')
TLS_CIPHER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].name}')

if [ -z "$TLS_MIN" ] && [ -z "$TLS_CIPHER" ]; then
  echo "✓ TLS env vars removed successfully"
else
  echo "✗ TLS env vars still present"
  oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o yaml | grep -A 2 TLS_
fi
```

**Expected Results**:
- Installer set hash annotation changed
- TektonResult `platform-data-hash` annotation removed
- Deployment rolled out
- `TLS_MIN_VERSION` and `TLS_CIPHER_SUITES` env vars NOT present
- APIServer profile remains set (not affected by feature toggle)

---

### Test 9: Re-enable Feature (Toggle Test)

**Objective**: Verify feature can be re-enabled cleanly.

**Steps**:
```bash
# 1. Ensure APIServer has a profile (or will use library-go defaults)
APISERVER_PROFILE=$(oc get apiserver cluster -o jsonpath='{.spec.tlsSecurityProfile.type}')
if [ -z "$APISERVER_PROFILE" ]; then
  echo "No explicit profile - will use library-go defaults"
else
  echo "APIServer profile: $APISERVER_PROFILE"
fi

# 2. Re-enable centralized TLS config
oc patch tektonconfig config --type=merge -p '{"spec":{"platforms":{"openshift":{"enableCentralTLSConfig":true}}}}'

# 3. Wait and verify
sleep 35
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 4. Verify TLS env vars re-injected
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
CIPHER_SUITES=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_CIPHER_SUITES")].value}')

if [ -n "$MIN_VERSION" ] && [ -n "$CIPHER_SUITES" ]; then
  echo "✓ Feature re-enabled successfully"
  echo "  TLS_MIN_VERSION: $MIN_VERSION"
  echo "  TLS_CIPHER_SUITES: ${CIPHER_SUITES:0:50}..."
else
  echo "✗ TLS env vars not re-injected"
fi
```

**Expected Results**:
- Feature re-enables cleanly
- TLS env vars re-injected (from APIServer profile or library-go defaults)
- No errors or stale state
- Deployment rolls out

---

## Edge Cases and Negative Tests

### Test 10: APIServer Profile Change While Feature Disabled

**Objective**: Verify changing APIServer profile when feature is disabled has no effect on TektonResult.

**Steps**:
```bash
# 1. Ensure feature disabled
oc patch tektonconfig config --type=merge -p '{"spec":{"platforms":{"openshift":{"enableCentralTLSConfig":false}}}}'
sleep 15

# 2. Record installer set and deployment state
HASH_BEFORE=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
REV_BEFORE=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

# 3. Change APIServer profile
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Modern","modern":{}}}}'

# 4. Wait to see if reconciliation triggered
sleep 45

# 5. Verify hash and deployment NOT updated
HASH_AFTER=$(oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o jsonpath='{.items[0].metadata.annotations.operator\.tekton\.dev/last-applied-hash}')
REV_AFTER=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}')

if [ "$HASH_BEFORE" = "$HASH_AFTER" ] && [ "$REV_BEFORE" = "$REV_AFTER" ]; then
  echo "✓ APIServer change did not trigger update (feature disabled)"
else
  echo "⚠ State changed despite feature being disabled"
  echo "  Hash: $HASH_BEFORE → $HASH_AFTER"
  echo "  Rev:  $REV_BEFORE → $REV_AFTER"
fi

# 6. Verify no TLS env vars
oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o yaml | grep TLS_MIN_VERSION
if [ $? -ne 0 ]; then
  echo "✓ No TLS env vars present (as expected)"
fi
```

**Expected Results**:
- APIServer profile change does NOT trigger TektonResult update
- Installer set hash annotation unchanged
- No TLS env vars injected
- Deployment revision unchanged

---

### Test 11: Rapid Profile Changes (Stability Test)

**Objective**: Verify operator handles rapid APIServer profile changes gracefully.

**Steps**:
```bash
# 1. Enable feature
oc patch tektonconfig config --type=merge -p '{"spec":{"platforms":{"openshift":{"enableCentralTLSConfig":true}}}}'
sleep 15

# 2. Rapidly change profiles
echo "Switching to Intermediate..."
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Intermediate","intermediate":{}}}}'
sleep 5

echo "Switching to Modern..."
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Modern","modern":{}}}}'
sleep 5

echo "Switching to Old..."
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Old","old":{}}}}'
sleep 5

echo "Switching back to Intermediate..."
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Intermediate","intermediate":{}}}}'

# 3. Wait for final reconciliation
echo "Waiting for system to stabilize..."
sleep 180

# 4. Verify system stabilized
READY=$(oc get tektonconfig config -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
echo "TektonConfig Ready: $READY"

if [ "$READY" = "True" ]; then
  echo "✓ TektonConfig is Ready"
else
  echo "✗ TektonConfig not ready: $READY"
fi

# 5. Check deployment is stable
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

# 6. Verify final TLS config matches last profile
MIN_VERSION=$(oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="TLS_MIN_VERSION")].value}')
echo "Final TLS_MIN_VERSION: $MIN_VERSION"

# Should match last profile (Intermediate = 1.2)
if [ "$MIN_VERSION" = "1.2" ]; then
  echo "✓ System stabilized with correct final state (Intermediate)"
else
  echo "⚠ Expected 1.2 (Intermediate), got: $MIN_VERSION"
fi

# 7. Check for errors in operator logs
echo "Checking for errors in operator logs..."
ERROR_COUNT=$(oc logs -n openshift-operators deployment/openshift-pipelines-operator --tail=200 | grep -i "error\|failed" | grep -v "level.*info" | wc -l)
echo "Error count in recent logs: $ERROR_COUNT"

if [ "$ERROR_COUNT" -eq 0 ]; then
  echo "✓ No errors in operator logs"
else
  echo "⚠ Found errors in operator logs - review manually"
fi
```

**Expected Results**:
- Operator handles rapid changes without crashing
- Final state matches last APIServer profile (Intermediate)
- TektonConfig Ready = True
- No stuck reconciliation loops
- No errors in operator logs
- Deployment eventually stabilizes

---

## Validation Checklist

After completing all tests, verify:

- [ ] Operator logs show no errors or warnings
- [ ] TektonConfig status is Ready
- [ ] TektonResult is functional (can store and retrieve pipeline results)
- [ ] Installer set `last-applied-hash` annotation updates correctly
- [ ] Deployment revisions increment as expected
- [ ] No orphaned installer sets or deployments
- [ ] APIServer resource can be restored to original state
- [ ] Feature can be toggled on/off multiple times without issues
- [ ] Rapid profile changes handled gracefully
- [ ] Library-go defaults behavior understood and verified
- [ ] Annotation-based trigger mechanism working correctly

---

## Troubleshooting Commands

```bash
# Check operator logs
oc logs -n openshift-operators deployment/openshift-pipelines-operator --tail=100 -f

# Check TektonConfig status
oc get tektonconfig config -o yaml | grep -A 20 'status:'

# Check TektonResult status and annotations
oc get tektonresult result -n $TEKTON_NAMESPACE -o yaml | grep -A 15 'status:\|annotations:'

# Check installer set details
oc get tektoninstallerset -n $TEKTON_NAMESPACE -l operator.tekton.dev/type=result -o yaml | grep -A 10 'annotations:'

# Check deployment details
oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o yaml | grep -A 20 'env:'

# Check events
oc get events -n $TEKTON_NAMESPACE --sort-by='.lastTimestamp' | tail -20

# Verify RBAC permissions
oc auth can-i get apiservers --as=system:serviceaccount:openshift-operators:openshift-pipelines-operator

# Force reconciliation of TektonConfig
oc annotate tektonconfig config reconcile-at=$(date +%s) --overwrite

# Force reconciliation of TektonResult
oc annotate tektonresult result -n $TEKTON_NAMESPACE reconcile-at=$(date +%s) --overwrite
```

---

## Cleanup

```bash
# 1. Disable feature
oc patch tektonconfig config --type=merge -p '{"spec":{"platforms":{"openshift":{"enableCentralTLSConfig":false}}}}'

# 2. Restore APIServer to default (or original) profile
# Check OpenShift default for your cluster version
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":null}}'

# 3. Wait for reconciliation
sleep 35

# 4. Verify cleanup
oc get $RESULT_DEPLOY -n $TEKTON_NAMESPACE -o yaml | grep TLS_
# Should return empty

# 5. Wait for final reconciliation
oc rollout status $RESULT_DEPLOY -n $TEKTON_NAMESPACE --timeout=2m

echo "Cleanup complete"
```

---

## Notes

### Annotation-Based Update Mechanism

The feature uses an annotation-based approach to trigger updates:

1. **TektonConfig level**: `GetPlatformData()` returns SHA-256 hash of TLS profile
2. **TektonResult CR**: Hash stamped as `operator.tekton.dev/platform-data-hash` annotation
3. **InstallerSet level**: Annotation value included in `last-applied-hash` computation
4. **Update trigger**: Annotation change → hash mismatch → installer set updated in-place

**Key insight**: The installer set **name stays the same** - it's updated in-place. Check the `last-applied-hash` annotation to verify updates.

### Library-Go Default Behavior

**Important**: When `enableCentralTLSConfig=true` and the APIServer has no explicit TLS profile (`tlsSecurityProfile: null`), library-go provides default TLS values:

- **TLS_MIN_VERSION**: `"1.2"`
- **TLS_CIPHER_SUITES**: 6 TLS 1.2 ciphers:
  - `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
  - `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
  - `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
  - `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
  - `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`
  - `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`

**Note**: Library-go defaults do NOT include TLS 1.3 ciphers. TLS 1.3 ciphers are only present when:
- Using predefined profiles (Intermediate, Modern, Old) - supplemented by operator code
- Using custom profile that explicitly lists TLS 1.3 ciphers

### General Notes

- All tests modify configuration only - no operator pod restarts required
- Typical reconciliation time is 10-30 seconds after config changes
- Recommended wait time is 35 seconds to account for APIServer watch trigger + reconciliation
- APIServer watch triggers reconciliation when TLS profile changes (while feature is enabled)
- **Predefined profiles** (Intermediate, Modern, Old) require the corresponding field:
  - `{"type":"Intermediate","intermediate":{}}`
  - `{"type":"Modern","modern":{}}`
  - `{"type":"Old","old":{}}`
- TLS 1.3 cipher supplementation adds TLS 1.3 ciphers to predefined profiles
- The `SKIP_APISERVER_TLS_WATCH` env var is for edge cases only and should NOT be used in testing
