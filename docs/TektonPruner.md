<!--
---
linkTitle: "TektonPruner"
weight: 30
---
-->

# Tekton Pruner

**TektonPruner** is a custom resource that enables users to install and manage the [Tekton Pruner][TektonPruner] component for event-driven cleanup of Tekton resources.

**Note:** It is recommended to install Tekton Pruner via the [TektonConfig](./TektonConfig.md) resource for consistency and ease of configuration.

**Important:** The job-based pruner must be disabled for the event-based pruner to be enabled. Both pruners cannot be enabled simultaneously.

---

## TektonPruner Custom Resource

Example of a `TektonPruner` Custom Resource (CR):

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonPruner
metadata:
  name: pruner
spec:
  targetNamespace: tekton-pipelines
  disabled: false
  global-config:
    enforcedConfigLevel: global
    historyLimit: 100
    ttlSecondsAfterFinished: 3600
    successfulHistoryLimit: 50
    failedHistoryLimit: 20
```

On OpenShift, the `targetNamespace` should be set to `openshift-pipelines`.

Check the status of installation:

```sh
kubectl get tektonpruners.operator.tekton.dev
```

---

## TektonConfig Pruner Configuration

Default configuration when using TektonConfig:

```yaml
pruner:
  disabled: true  # Disable job-based pruner

tektonpruner:
  disabled: false
  global-config:
    enforcedConfigLevel: global
    historyLimit: 100
    ttlSecondsAfterFinished: null
    successfulHistoryLimit: null
    failedHistoryLimit: null
  options: {}
```

### Configuration Levels

- **`global`**: Cluster-wide defaults apply to all namespaces (no namespace overrides)
- **`namespace`**: Allows namespace-level overrides via ConfigMaps

### Pruning Fields

The pruner deletes runs when any one of the specified conditions is met:

- **`historyLimit`**: Maximum number of runs to retain for each status (applies independently to both successful and failed runs)
- **`successfulHistoryLimit`**: Maximum number of successful runs to retain
- **`failedHistoryLimit`**: Maximum number of failed runs to retain
- **`ttlSecondsAfterFinished`**: Time in seconds to retain completed runs before pruning

**Note:** If `successfulHistoryLimit` or `failedHistoryLimit` is not specified, `historyLimit` value is used as fallback.

### Example Configuration

```yaml
tektonpruner:
  disabled: false
  global-config:
    enforcedConfigLevel: namespace
    historyLimit: 100
    ttlSecondsAfterFinished: 3600
    namespaces:
      dev:
        historyLimit: 50
        ttlSecondsAfterFinished: 1800
      prod:
        successfulHistoryLimit: 200
        failedHistoryLimit: 50
```

---

## Namespace-Level Configuration

When `enforcedConfigLevel` is set to `namespace`, individual namespaces can override pruning settings using a ConfigMap.

**Configuration Precedence:** If a namespace has configurations defined in both the global config (under `namespaces` section) and a namespace-level ConfigMap, the namespace-level ConfigMap values take precedence.

Create a ConfigMap named `tekton-pruner-namespace-spec` in the target namespace:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tekton-pruner-namespace-spec
  namespace: my-namespace
data:
  ns-config: |
    historyLimit: 100
    ttlSecondsAfterFinished: 7200
    pipelineRuns:
      - name: critical-pipeline
        successfulHistoryLimit: 500
        failedHistoryLimit: 100
      - selector:
          - matchLabels:
              env: production
        ttlSecondsAfterFinished: 604800
    taskRuns:
      - name: cleanup-task
        historyLimit: 10
      - selector:
          - matchLabels:
              type: test
        ttlSecondsAfterFinished: 3600
```

---

## Learn More

For detailed configuration options, tutorials, and advanced use cases, refer to the [Tekton Pruner Getting Started Guide][GettingStarted].

For source code and additional information, visit the [Tekton Pruner GitHub repository][TektonPruner].

---

[TektonPruner]: https://github.com/tektoncd/pruner
[GettingStarted]: https://github.com/tektoncd/pruner/blob/main/docs/tutorials/getting-started.md
