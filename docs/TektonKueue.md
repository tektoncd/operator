# Tekton Kueue

The cluster-scoped `TektonKueue` custom resource installs and manages
[Tekton Kueue](https://github.com/tektoncd/tekton-kueue). Using
[`TektonConfig`](./TektonConfig.md) is recommended.

## Prerequisites

Install these APIs before enabling Tekton Kueue:

- [Kueue](https://kueue.sigs.k8s.io/)
- [cert-manager](https://cert-manager.io/docs/installation/)

## Configuration

Enable Tekton Kueue through `TektonConfig`:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  kueue:
    disabled: false
    multi-cluster-disabled: true
    multi-cluster-role: ""
    config.yaml:
      queueName: pipelines-queue
```

Alternatively, create the component resource directly:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonKueue
metadata:
  name: kueue
spec:
  disabled: false
  multi-cluster-disabled: true
  multi-cluster-role: ""
  config.yaml:
    queueName: pipelines-queue
```

## Multi-cluster configuration

A multi-cluster installation can configure the cluster as a Hub or Spoke.

Hub:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonKueue
metadata:
  name: kueue
spec:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Hub
  config.yaml:
    queueName: pipelines-queue
```

When the Hub role is configured through `TektonConfig`, the operator also
manages the [Syncer Service](./SyncerService.md) and
[Tekton Multicluster Proxy AAE](./TektonMulticlusterProxyAAE.md).

Spoke:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonKueue
metadata:
  name: kueue
spec:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Spoke
  config.yaml:
    queueName: pipelines-queue
```

## Upgrade compatibility

`TektonKueue` replaces the former `TektonScheduler` API. The deprecated
`TektonScheduler` CRD and `TektonConfig.spec.scheduler` field remain available
during the compatibility window.

During pre-upgrade reconciliation, the operator:

1. Copies `TektonConfig.spec.scheduler` to `TektonConfig.spec.kueue` and clears
   the deprecated field. If both fields are configured, `spec.kueue` wins.
2. Removes InstallerSets owned by the old component.
3. Removes the deprecated `tekton-scheduler-role` and
   `tekton-scheduler-rolebinding` resources.
4. Creates `TektonKueue/kueue` from an existing `TektonScheduler/scheduler`
   resource when a `TektonKueue` resource does not already exist.
5. Removes the old operator finalizer, preserves other finalizers, and requests
   deletion of the migrated legacy resource.

The deprecated API is retained only for migration and does not reconcile
operand resources. Use `TektonKueue` and `spec.kueue` for new configuration.

Rename `IMAGE_SCHEDULER_MANAGER` and `IMAGE_SCHEDULER_WEBHOOK` overrides to
`IMAGE_KUEUE_MANAGER` and `IMAGE_KUEUE_WEBHOOK`.
