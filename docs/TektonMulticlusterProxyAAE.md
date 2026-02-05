<!--
---
linkTitle: "TektonMulticlusterProxyAAE"
weight: 12
---
-->
# Tekton Multicluster Proxy AAE

TektonMulticlusterProxyAAE deploys the **multicluster-proxy-aae** component used in multi-cluster setups with [Tekton Scheduler](./TektonScheduler.md) and [Kueue](https://kueue.sigs.k8s.io/). The proxy runs on the **Hub** cluster and communicates with **Spoke** clusters (e.g. via [MultiKueue](https://kueue.sigs.k8s.io/docs/concepts/multikueue/)).

It is recommended to install this component through [TektonConfig](./TektonConfig.md) by enabling the scheduler with multi-cluster and setting the cluster role to **Hub**. TektonConfig will create and reconcile the TektonMulticlusterProxyAAE CR automatically; you do not need to create it manually when using TektonConfig.

## When is TektonMulticlusterProxyAAE installed?

TektonConfig creates and manages TektonMulticlusterProxyAAE only when all of the following are true:

- Scheduler is **not** disabled (`spec.scheduler.disabled` is not `true`).
- Multi-cluster is **enabled** (`spec.scheduler.multi-cluster-disabled` is `false`).
- Cluster role is **Hub** (`spec.scheduler.multi-cluster-role` is `Hub`).

On **Spoke** clusters, leave `multi-cluster-role` as `Spoke`; the proxy is not installed there.

## Prerequisites

The proxy is only installed when the [Tekton Scheduler](./TektonScheduler.md) is enabled with multi-cluster (Hub role). **cert-manager** and **Kueue** (and **MultiKueue** for the proxy) are required for the Scheduler and multi-cluster in general; you must install them before enabling the scheduler. For full prerequisites and multi-cluster configuration, see [Tekton Scheduler](./TektonScheduler.md). [TektonConfig](./TektonConfig.md) (Scheduler section) also summarizes this and points to Tekton Scheduler.

If you see a config status error such as:

```
Components not in ready state: Please install cert-manager (cert-manager.io/v1) First, the server could not find the requested resource
```

cert-manager is not installed or its APIs are not available. Install [cert-manager](https://cert-manager.io/docs/installation/) (v1 API) and ensure [Kueue](https://kueue.sigs.k8s.io/) (and for the proxy, MultiKueue) are installed as described in [Tekton Scheduler](./TektonScheduler.md), then re-apply or reconcile TektonConfig.

- The proxy’s readiness endpoint returns ready only when at least one worker cluster is registered; ensure you have at least one valid `MultiKueueCluster` (and corresponding kubeconfig secret in `kueue-system`) if you want the proxy to report Ready.

## TektonMulticlusterProxyAAE CR

The TektonMulticlusterProxyAAE CR is cluster-scoped. When managed by TektonConfig, the operator creates a single instance named `multicluster-proxy-aae`.

Example (for reference; TektonConfig creates this when scheduler multi-cluster Hub is enabled):

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonMulticlusterProxyAAE
metadata:
  name: multicluster-proxy-aae
spec:
  targetNamespace: tekton-pipelines   # or openshift-pipelines on OpenShift
  options: {}
```

### Properties

- **targetNamespace**: Namespace where the proxy deployment and related resources are installed (e.g. `tekton-pipelines` or `openshift-pipelines`). Set via TektonConfig when the CR is created by the operator.
- **options**: Optional [AdditionalOptions](./TektonConfig.md#additional-fields-as-options) for customizing deployments, ConfigMaps, or webhook configuration.

## Checking installation status

```sh
kubectl get tektonmulticlusterproxyaaes.operator.tekton.dev
# or
oc get tektonmulticlusterproxyaaes.operator.tekton.dev
```

Check the proxy deployment and pods in the target namespace (e.g. `openshift-pipelines` or `tekton-pipelines`):

```sh
kubectl get deployment -n <target-namespace> -l app.kubernetes.io/component=proxy-aae
kubectl get pods -n <target-namespace> -l app.kubernetes.io/component=proxy-aae
```

## Standalone installation

If you are not using TektonConfig (e.g. you manage scheduler and multi-cluster yourself), you can create a TektonMulticlusterProxyAAE CR manually. Ensure the cluster has Kueue and MultiKueue APIs and that the operator is installed; the operator will reconcile the CR and deploy the proxy in the specified `targetNamespace`.

## Related

- [TektonConfig](./TektonConfig.md) – Scheduler and TektonMulticlusterProxyAAE section
- [Tekton Scheduler](./TektonScheduler.md) – Multi-cluster configuration (Hub / Spoke)
- [Kueue](https://kueue.sigs.k8s.io/) and [MultiKueue](https://kueue.sigs.k8s.io/docs/concepts/multikueue/)
