<!--
---
linkTitle: "ShipwrightBuild"
weight: 14
---
-->
# ShipwrightBuild

ShipwrightBuild custom resource allows the user to install and manage
[Shipwright Build][shipwright], a framework for building container images on
Kubernetes using tools such as Buildah, Buildpacks, and Kaniko.

It is recommended that you install ShipwrightBuild through [TektonConfig](./TektonConfig.md).

> **NOTE:** ShipwrightBuild depends on
> [TektonPipeline](./TektonPipeline.md) being installed and ready, since Shipwright
> runs builds as Tekton `TaskRuns`.

> **NOTE (Kubernetes only):** On Kubernetes, [cert-manager](https://cert-manager.io/)
> must be installed **before** enabling ShipwrightBuild. The operator uses
> cert-manager to provision the Shipwright webhook's serving certificate and to
> inject the CA bundle into the Shipwright conversion-webhook CRDs (via the
> `cert-manager.io/inject-ca-from` annotation, populated by cert-manager's
> cainjector). If cert-manager is not installed, the ShipwrightBuild CR will not
> become ready (its status reports the missing `cert-manager.io/v1` API).
>
> This dependency applies to Kubernetes only. On OpenShift the serving
> certificate and CA bundle are handled by the built-in service-ca operator, so
> cert-manager is **not** required there.

- ShipwrightBuild CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: ShipwrightBuild
    metadata:
      name: shipwright-build
    spec:
      enabled: true
      targetNamespace: openshift-pipelines
    ```

- Check the status of the installation using following command:

    ```sh
    kubectl get shipwrightbuilds.operator.tekton.dev
    ```

## ShipwrightBuild Config

The recommended way to update the ShipwrightBuild CR is using [TektonConfig](./TektonConfig.md) CR.

### Properties (Mandatory)

 - `targetNamespace`

    Set this field to provide the namespace in which you want to install the
    ShipwrightBuild component.

### Properties (Optional)

The fields have default values so even if the user has not passed them in the CR,
the operator will add them. The user can later change them as per their need.

 - `enabled`

    Enables or disables installation of the ShipwrightBuild component. Defaults to
    `false`.

 - `config`

    Holds configuration applied to the resources created by ShipwrightBuild.

    | Field | Description |
    |-------|-------------|
    | `nodeSelector` | Node selector applied to the component's pod templates. |
    | `tolerations` | Tolerations applied to the component's pod templates. |
    | `priorityClassName` | Priority class set on the component's pod templates. |

    Example:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: ShipwrightBuild
    metadata:
      name: shipwright-build
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
    resources such as Deployments, ConfigMaps, and more.

#### NetworkPolicy

`networkPolicy` configures the [NetworkPolicy][np] resources the operator creates
for the ShipwrightBuild workloads (the controller Deployment and the webhook
Deployment). By default, the operator ships a `default-deny` policy plus additive
per-workload policies.

| Field | Description |
|-------|-------------|
| `disabled` | When `true`, disables all NetworkPolicy creation for this component. Existing policies are removed on the next reconcile. Defaults to `false`. |
| `policies` | Merges with the operator's default NetworkPolicies by name. A key matching a default policy name replaces that default entirely; a key not matching any default is added alongside the defaults. If empty, all operator defaults are applied unchanged. |

Example — disabling managed NetworkPolicies:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: ShipwrightBuild
metadata:
  name: shipwright-build
spec:
  targetNamespace: openshift-pipelines
  networkPolicy:
    disabled: true
```

For more details, see the [NetworkPolicy](./NetworkPolicy.md) documentation.

## Singleton Instance

Only a single instance of ShipwrightBuild is allowed on the cluster. The instance
must be named `shipwright-build`; the validating webhook rejects any other name.

[shipwright]:https://github.com/shipwright-io/build
[np]:https://kubernetes.io/docs/concepts/services-networking/network-policies/
