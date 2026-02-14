# Syncer Service

SyncerService custom resource allows user to install and manage [Syncer Service][SyncerService].

Syncer Service is a Kubernetes controller that synchronizes secrets between manager (hub) and worker (spoke) nodes in multi-Kueue environments. It enables seamless multi-cluster pipeline execution by ensuring PipelineRuns have the necessary authentication secrets available on their target clusters.

## Overview

When a PipelineRun is scheduled to run on a spoke cluster via Kueue MultiKueue:

1. The controller detects the Workload resource associated with the PipelineRun
2. Retrieves the Git authentication secret specified in the PipelineRun's annotations
3. Syncs the secret from the hub cluster to the target spoke cluster
4. Ensures the secret has proper ownership for lifecycle management

## Installation

**Important:** SyncerService is **not installed directly by users**. It is automatically deployed by the Tekton Operator when specific conditions are met.

### Automatic Installation Conditions

SyncerService is automatically installed when **both** of the following conditions are true:

1. Multi-cluster mode is **enabled** (`multi-cluster-disabled: false`)
2. Multi-cluster role is set to **Hub** (`multi-cluster-role: Hub`)

### Installation via TektonConfig

Configure your TektonConfig with the scheduler settings to automatically deploy SyncerService:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  profile: all
  targetNamespace: openshift-pipelines
  scheduler:
    disabled: false
    multi-cluster-disabled: false
    multi-cluster-role: Hub
```

When this configuration is applied, the operator will:
1. Deploy TektonScheduler component
2. Automatically deploy SyncerService component (because `multi-cluster-role: Hub`)

### Installation via TektonScheduler CR

Alternatively, you can configure via TektonScheduler CR directly:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonScheduler
metadata:
  name: scheduler
spec:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Hub
```

## Uninstallation

### Automatic Uninstallation Conditions

SyncerService is automatically removed when **any** of the following conditions become true:

1. Multi-cluster mode is **disabled** (`multi-cluster-disabled: true`)
2. Multi-cluster role is **not Hub** (e.g., `Spoke` or empty/unset)

### Uninstall via TektonConfig

To remove SyncerService, update your TektonConfig to disable multi-cluster mode or change the role:

**Option 1: Disable Multi-Cluster**
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  scheduler:
    multi-cluster-disabled: true
```

**Option 2: Change to Spoke Role**
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  scheduler:
    multi-cluster-disabled: false
    multi-cluster-role: Spoke
```

## Pre-Requisites

SyncerService has the same pre-requisites as TektonScheduler:

* [Kueue](https://kueue.sigs.k8s.io) must be installed
* [cert-manager](https://github.com/cert-manager/cert-manager) CRDs must be installed

## Features

* **Automatic Secret Syncing**: Syncs Git authentication secrets from hub to spoke clusters
* **Multi-Cluster Support**: Works with Kueue's MultiKueue for distributed workload execution
* **Selective Processing**: Only handles Workloads owned by Tekton PipelineRuns
* **Lifecycle Management**: Automatically managed by the Tekton Operator based on scheduler configuration

## SyncerService CR

The SyncerService CR is managed internally by the operator and looks like:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: SyncerService
metadata:
  name: syncer-service
spec:
  targetNamespace: openshift-pipelines
```

> **Note:** Users should not create or modify SyncerService CR directly. It is managed automatically by the operator based on TektonScheduler configuration.

## Troubleshooting

### Check SyncerService Status

```bash
kubectl get syncerservice syncer-service -o yaml
```

### Check if SyncerService Should Be Installed

Verify your scheduler configuration:

```bash
kubectl get tektonconfig config -o jsonpath='{.spec.scheduler}'
```

For SyncerService to be deployed, you should see:
- `multi-cluster-disabled: false`
- `multi-cluster-role: Hub`

### View Controller Logs

```bash
kubectl logs -n openshift-pipelines -l app=syncer-service -f
```

## References

* [Syncer Service Repository](https://github.com/openshift-pipelines/syncer-service)
* [Kueue Documentation](https://kueue.sigs.k8s.io)
* [Kueue MultiKueue](https://kueue.sigs.k8s.io/docs/concepts/multikueue/)
* [Tekton Pipelines](https://tekton.dev)
* [TektonScheduler Documentation](./TektonScheduler.md)

[SyncerService]: https://github.com/openshift-pipelines/syncer-service
