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

## Fleet manager

`fleet-manager` selects how worker clusters are connected to MultiKueue:

- `manual` (default when omitted): administrators or external tooling manage
  cluster connections. The operator does not install a plugin.
- `acm`: on an OpenShift hub, the operator installs the
  [Pipelines MultiKueue Plugin](https://github.com/openshift-pipelines/pipelines-multikueue-plugin).
  Tekton Kueue and multi-cluster mode must be enabled.

For example, configure an ACM hub through TektonConfig:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  kueue:
    disabled: false
    multi-cluster-disabled: false
    multi-cluster-role: Hub
    fleet-manager: acm
    config.yaml:
      queueName: pipelines-queue
```

For a standalone TektonKueue, set the same fields directly under `spec`.
Existing installations default to `manual`.

The hub requires Kueue, cert-manager, OLM, the OpenShift Kueue API, and the OCM
ManagedCluster, ManifestWork and ManagedServiceAccount APIs. The plugin release
uses `kueue.x-k8s.io/v1beta2`. Its ManagedServiceAccount add-on must also be running.

The current plugin enrolls all non-local ManagedClusters, bootstraps operators on
workers, and creates its default hub queue resources. These behaviors are defined
by the bundled plugin release. Its default LocalQueue is `pipelines-queue` in
`default`; changing TektonKueue's `config.yaml.queueName` does not reconfigure the
plugin's queue names.

The operator installs the plugin in `targetNamespace` through a Custom InstallerSet
owned by TektonKueue. TektonKueue readiness includes plugin installation readiness,
not the connectivity of every worker. Selecting `manual`, disabling the parent or
multi-cluster mode, or deleting TektonKueue removes the plugin installation.
Operators, queues and connections provisioned by the plugin remain. After removal,
administrators or external tooling must maintain connection credentials.

The operator sets the plugin's `KUEUE_NAMESPACE` environment variable to
`openshift-kueue-operator`, independently of `targetNamespace`. Existing
`spec.kueue.options.deployments.pipelines-multikueue-plugin-controller` options
can customize the `controller` container, including supported environment
variables. For standalone TektonKueue, use `spec.options.deployments`.
`KUEUE_NAMESPACE` changes Secret placement only; the bundled external Kueue RBAC
binding still targets `openshift-kueue-operator`.

Operator deployments can override the plugin image with
`IMAGE_MULTIKUEUE_PLUGIN_CONTROLLER`. The plugin also uses the existing
`TEKTON_REGISTRY_OVERRIDE` mechanism.

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
