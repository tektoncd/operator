<!--
---
linkTitle: "TektonPruner"
weight: 30
---
-->

# Tekton Pruner

**TektonPruner** is a custom resource that enables users to install and manage the [Tekton Pruner][TektonPruner] component.

> ℹ️ It is recommended to install Tekton Pruner via the [TektonConfig](./TektonConfig.md) resource for consistency and ease of configuration.

> ℹ️ Job based pruner **MUST** be disabled for the new event based pruner to be enabled.

---

## TektonPruner Custom Resource

Here is an example of a `TektonPruner` Custom Resource (CR):

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
    ttlSecondsAfterFinished: null
    successfulHistoryLimit: null
    failedHistoryLimit: null
    historyLimit: 100
```

On OpenShift, the `targetNamespace` should be set to `openshift-pipelines`.

- Check the status of installation using following command:

    ```sh
    kubectl get tektonpruners.operator.tekton.dev
    ```


You can configure and install the pruner using `TektonConfig` .

---

## TektonConfig Pruner Configuration
Here is the default configuration:

```yaml
tektonpruner:
  disabled: true
  global-config:
    enforcedConfigLevel: global
    failedHistoryLimit: null
    historyLimit: 100
    successfulHistoryLimit: null
    ttlSecondsAfterFinished: null
  options: {}
```
### Enabling Tekton Pruner

Tekton Pruner is now available as part of `TektonConfig`. You can manage its settings under the `tektonpruner` section.

TektonPruner is disabled by default and to enable Tekton Pruner, set `disabled` to `false` in the `tektonpruner` section of your `TektonConfig`:
At the same time you also need to disable the older pruner. Both pruners cannot be enabled at the same time.

```yaml
pruner:
  disabled: true
  
tektonpruner:
  disabled: false
```
If you try to enable both pruners, you will encounter an error during installation.

### 🔧 Customizing History Retention
You can fine-tune pruning behavior by modifying the following fields under `global-config`:

- `historyLimit`: Maximum number of total completed runs to retain.
- `failedHistoryLimit`: Number of failed runs to retain.
- `successfulHistoryLimit`: Number of successful runs to retain.
- `ttlSecondsAfterFinished`**: Time (in seconds) to retain completed runs before pruning.

> You can specify any combination of these fields. The pruner deletes runs when **any one** of the specified conditions is met.

#### Example:

```yaml
global-config:
  ttlSecondsAfterFinished: 3600
  historyLimit: 100
```

This configuration deletes the runs if:
- They are older than **1 hour**, OR
- There are more than **100** successful runs, OR
- There are more than **100** failed runs

---

## Learn More

For more details and source code, visit the [Tekton Pruner GitHub repository][TektonPruner].

---

[TektonPruner]: https://github.com/tektoncd/pruner
