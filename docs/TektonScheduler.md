# Tekton Scheduler

TektonScheduler custom resource allows user to install and manage [Tekton Scheduler][Scheduler].

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonScheduler CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonScheduler
metadata:
  name: Scheduler
spec:
  disabled: true
```

### Pre-Requisite

* Scheduler component  internally uses  [Kueue](:https://kueue.sigs.k8s.io)  for its  functioning so it is required to have kueue installed before we can enable the tekton-scheduler.
* Scheduler component also uses [cert-manager](:https://github.com/cert-manager/cert-manager) so you must install the cert-manager CRDs before scheduler can be enabled.

### Enable Scheduler

Scheduler component can be enabled by  setting the  disabled to false in TektonConfig

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonScheduler
metadata:
  name: Scheduler
spec:
  disabled: false
```

# Multi Cluster Configuration

If you are working with multi-cluster pipelines then you can enable the same from tekton scheduler config.

in multi-cluster environment a cluster can play the role of Hub or Spoke. The TektonConfig  settings for Scheduler  for Hub and Spoke is defined as below


### Hub Cluster

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonScheduler
metadata:
  name: Scheduler
spec:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Hub
```

> **Note:** When `multi-cluster-role: Hub` is configured, the operator automatically deploys the [syncer-service](https://github.com/openshift-pipelines/syncer-service) component for multi-cluster synchronization.

### Spoke Cluster

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonScheduler
metadata:
  name: Scheduler
spec:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Spoke  
```

You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

[Scheduler](:https://github.com/konflux-ci/tekton-kueue)
