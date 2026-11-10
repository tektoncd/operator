<!--
---
linkTitle: "SharedResource"
weight: 13
---
-->
# SharedResource

SharedResource custom resource allows the user to install and manage the
[Shared Resource CSI Driver][shared-resources], which lets Tekton workloads mount
cluster `Secret` and `ConfigMap` resources as CSI volumes in a controlled,
namespace-scoped way.

It is recommended that you install SharedResource through [TektonConfig](./TektonConfig.md).

> **NOTE:** SharedResource is an OpenShift-only component. It is reconciled by the
> OpenShift platform reconciler and is not installed on vanilla Kubernetes.

- SharedResource CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: SharedResource
    metadata:
      name: shared-resource
    spec:
      enabled: true
      targetNamespace: openshift-pipelines
    ```

- Check the status of the installation using following command:

    ```sh
    kubectl get sharedresources.operator.tekton.dev
    ```

## SharedResource Config

The recommended way to update the SharedResource CR is using [TektonConfig](./TektonConfig.md) CR.

### Properties (Mandatory)

 - `targetNamespace`

    Set this field to provide the namespace in which you want to install the
    SharedResource component.

### Properties (Optional)

The fields have default values so even if the user has not passed them in the CR,
the operator will add them. The user can later change them as per their need.

 - `enabled`

    Enables or disables installation of the SharedResource component. Defaults to
    `true`. Setting it to `false` removes the component's resources on the next
    reconcile.

 - `config`

    Holds configuration applied to the resources created by SharedResource.

    | Field | Description |
    |-------|-------------|
    | `nodeSelector` | Node selector applied to the component's pod templates. |
    | `tolerations` | Tolerations applied to the component's pod templates. |
    | `priorityClassName` | Priority class set on the component's pod templates. |

    Example:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: SharedResource
    metadata:
      name: shared-resource
    spec:
      targetNamespace: openshift-pipelines
      config:
        nodeSelector:
          kubernetes.io/os: linux
        tolerations:
          - key: "node-role.kubernetes.io/infra"
            operator: "Exists"
            effect: "NoSchedule"
        priorityClassName: system-cluster-critical
    ```

 - `options`

    Holds additional fields that are merged onto the generated manifests
    (`AdditionalOptions`), allowing advanced customization of the created
    resources such as Deployments, DaemonSets, ConfigMaps, and more.

#### NetworkPolicy

`networkPolicy` configures the [NetworkPolicy][np] resources the operator creates
for the SharedResource workloads (the CSI driver node DaemonSet and the webhook
Deployment). By default, the operator ships a `default-deny` policy plus additive
per-workload policies.

| Field | Description |
|-------|-------------|
| `disabled` | When `true`, disables all NetworkPolicy creation for this component. Existing policies are removed on the next reconcile. Defaults to `false`. |
| `policies` | Merges with the operator's default NetworkPolicies by name. A key matching a default policy name replaces that default entirely; a key not matching any default is added alongside the defaults. If empty, all operator defaults are applied unchanged. |

Example — disabling managed NetworkPolicies:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: SharedResource
metadata:
  name: shared-resource
spec:
  targetNamespace: openshift-pipelines
  networkPolicy:
    disabled: true
```

For more details, see the [NetworkPolicy](./NetworkPolicy.md) documentation.

## Singleton Instance

Only a single instance of SharedResource is allowed on the cluster. The instance
must be named `shared-resource`; the validating webhook rejects any other name.

[shared-resources]:https://github.com/openshift/csi-driver-shared-resource
[np]:https://kubernetes.io/docs/concepts/services-networking/network-policies/
