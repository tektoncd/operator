# PostgreSQL Upgrade from Version 13 to 15

## Overview

This document describes the automatic PostgreSQL upgrade mechanism implemented in the Tekton Operator to handle upgrades from PostgreSQL 13 to PostgreSQL 15 for the Tekton Results internal database. This only applies for OpenShift deployments. Kubernetes deployments use Bitnami Postgres images which do not have the same versioning issue.

## Problem Statement

When upgrading the operator from a version using PostgreSQL 13 to a version using PostgreSQL 15, the PostgreSQL pod would fail to start because:

1. PostgreSQL 15 cannot directly read the data directory structure created by PostgreSQL 13
2. The sclorg PostgreSQL container supports upgrades via the `POSTGRESQL_UPGRADE` environment variable, but this must only be set **once**
3. If the env var remains set after the initial upgrade, subsequent pod restarts will fail
4. The operator reconciliation loop continuously reapplies manifests, making it difficult to set a "one-time" environment variable

## Solution: Wrapper Script Approach

The solution uses a **wrapper script** that automatically detects when an upgrade is needed and applies the `POSTGRESQL_UPGRADE` environment variable exactly once.

### Key Components

1. **ConfigMap with Wrapper Script** (`postgres-upgrade-scripts`)
   - `postgres-wrapper.sh`: Checks PostgreSQL version and conditionally sets upgrade env var

2. **Modified Main Container**
   - Uses wrapper script as entrypoint instead of default `run-postgresql`
   - Wrapper checks `PG_VERSION` file in data directory
   - Sets `POSTGRESQL_UPGRADE=copy` if version 13 is detected
   - Executes normal PostgreSQL startup

### How It Works

```
Pod Starts
    ↓
Main Container: postgres
    ↓
Wrapper Script Executes
    ↓
Check /var/lib/pgsql/data/userdata/PG_VERSION
    ↓
    ├─ If file doesn't exist → Fresh install (no action)
    ├─ If version = 15 → Start normally (no action)
    └─ If version = 13 → Set POSTGRESQL_UPGRADE=copy
    ↓
Execute run-postgresql
    ↓
PostgreSQL starts (with or without upgrade)
    ↓
PG_VERSION updated to 15 after successful upgrade
    ↓
Next restart → Version = 15 → Start normally
```

## Implementation Details

### Files Modified

1. **`cmd/openshift/operator/kodata/static/tekton-results/internal-db/db.yaml`**
   - Changed PostgreSQL image from version 16 to version 15
   - Added `postgres-upgrade-scripts` ConfigMap with wrapper script

2. **`pkg/reconciler/openshift/tektonresult/extension.go`**
   - Added `injectPostgresUpgradeSupport()` transformer
   - Transformer modifies postgres container to use wrapper script

3. **`pkg/reconciler/kubernetes/tektonresult/transform.go`**
   - Added same `injectPostgresUpgradeSupport()` transformer for Kubernetes platform

### Transformer Behavior

The `injectPostgresUpgradeSupport()` transformer:

1. **Only applies to** the `tekton-results-postgres` StatefulSet
2. **Modifies main container** to:
   - Mount the scripts ConfigMap at `/upgrade-scripts`
   - Use wrapper script as command: `/bin/bash /upgrade-scripts/postgres-wrapper.sh`
3. **Adds ConfigMap volume** for the upgrade scripts

## Rollback Considerations

⚠️ **Important**: This is a **one-way upgrade**. Once the PostgreSQL data has been upgraded from v13 to v15, you cannot downgrade back to v13.

If you need to rollback the operator to a version using PostgreSQL 13:
1. The PostgreSQL 13 pod will fail to start with the upgraded data
2. You must either:
   - Restore from a backup taken before the upgrade
   - Accept data loss and start fresh

**Recommendation**: Always take a backup of the PostgreSQL PVC before upgrading the operator.

## References

- [PostgreSQL Container Upgrade Documentation](https://github.com/sclorg/postgresql-container/tree/master/15#upgrading-database)
- [sclorg PostgreSQL Container Repository](https://github.com/sclorg/postgresql-container)
- [Tekton Operator Documentation](https://github.com/tektoncd/operator)
