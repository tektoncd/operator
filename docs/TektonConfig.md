<!--
---
linkTitle: "TektonConfig"
weight: 1
---
-->
# Tekton Config

TektonConfig custom resource is the top most component of the operator which allows user to install and customize all other
components at a single place.

Operator provides support for installing and managing following operator components through `TektonConfig`:

- [TektonPipeline](./TektonPipeline.md)
- [TektonTrigger](./TektonTrigger.md)

Other than the above components depending on the platform operator also provides support for

- On Kubernetes
    - [TektonDashboard](./TekonDashboard.md)
- On OpenShift
    - [TektonAddon](./TektonAddon.md)

The TektonConfig CR provides the following features

```yaml
  apiVersion: operator.tekton.dev/v1alpha1
  kind: TektonConfig
  metadata:
    name: config
  spec:
    targetNamespace: tekton-pipelines
    profile: all
    config:
      nodeSelector: <>
      tolerations: []
    pipeline:
      disable-affinity-assistant: false
      disable-creds-init: false
      disable-home-env-overwrite: true
      disable-working-directory-overwrite: true
      enable-api-fields: stable
      enable-custom-tasks: false
      enable-tekton-oci-bundles: false
      metrics.pipelinerun.duration-type: histogram
      metrics.pipelinerun.level: pipelinerun
      metrics.taskrun.duration-type: histogram
      metrics.taskrun.level: taskrun
      require-git-ssh-secret-known-hosts: false
      running-in-environment-with-injected-sidecars: true
    pruner:
      resources:
      - taskrun
      - pipelinerun
      keep: 3
      schedule: "* * * * *"
    hub:
      params:
        - name: enable-devconsole-integration
          value: "true"
    dashboard:
      readonly: true
```
Look for the particular section to understand a particular field in the spec.

### Target Namespace

This allows user to choose a namespace to install the Tekton Components such as pipelines, triggers.

By default, namespace would be `tekton-pipelines` for Kubernetes and `openshift-pipelines` for OpenShift.

### Profile

This allows user to choose which all components to install on the cluster.
There are 3 profiles available:
- `all`: This profile will install all components
- `basic`:  This profile will install only TektonPipeline and TektonTrigger component
- `lite`: This profile will install only TektonPipeline component

On Kubernetes, `all` profile will install `TektonDashboard` and on OpenShift `TektonAddon` will be installed.


### Config

Config provides fields to configure deployments created by the Operator.
This provides following fields:
- [`nodeSelector`][node-selector]
- [`tolerations`][tolerations]

User can pass the required fields and this would be passed to all Operator components which will get added in all
deployments created by Operator.

Example:
```yaml
config:
  nodeSelector:
    key: value
  tolerations:
  - key: "key1"
    operator: "Equal"
    value: "value1"
    effect: "NoSchedule"
```

This is an `Optional` section.


### Pipeline
Pipeline section allows user to customize the Tekton pipeline features. This allow user to customize the values in configmaps.

Refer to [properties](./TektonPipeline.md#properties) section in TektonPipeline for available options.

Some of the fields have default values, so operator will add them if user haven't passed in CR. Other fields which
doesn't have default values won't be passed unless user specify them in CR. User can find those [here](./TektonPipeline.md#optional-properties).

Example:

```yaml
pipeline:
  disable-affinity-assistant: false
  disable-creds-init: false
  disable-home-env-overwrite: true
  disable-working-directory-overwrite: true
  enable-api-fields: stable
  enable-custom-tasks: false
  enable-tekton-oci-bundles: false
  metrics.pipelinerun.duration-type: histogram
  metrics.pipelinerun.level: pipelinerun
  metrics.taskrun.duration-type: histogram
  metrics.taskrun.level: taskrun
  require-git-ssh-secret-known-hosts: false
  running-in-environment-with-injected-sidecars: true
```

### Pruner
Pruner provides auto clean up feature for the Tekton resources.

Example:
```yaml
pruner:
  resources:
    - taskrun
    - pipelinerun
  keep: 3
  schedule: "* * * * *"
```

- `resources`: supported resources for auto prune are `taskrun` and `pipelinerun`
- `keep`: maximum number of resources to keep while deleting removing
- `schedule`: how often to clean up resources. User can understand the schedule syntax [here][schedule].

This is an `Optional` section.

### Addon

TektonAddon install some resources along with Tekton Pipelines on the cluster. This provides few ClusterTasks, PipelineTemplates.

This section allows to customize installation of those resources through params. You can read more about the supported params [here](./TektonAddon.md).

Example:
```yaml
addon:
  params:
    - name: "clusterTask"
      value: "true"
    - name: "pipelineTemplates"
      value: "true"
```

**NOTE**: TektonAddon is currently available for OpenShift Platform only. Enabling this for Kubernetes platform is in roadmap
of Operator.

### Hub

This is to enable/disable showing hub resources in pipeline builder of devconsole(OpenShift UI). By default, the field is
not there in the config object. If you want to disable the integration, you can add the param like below in config with value `false`. 
The possible values are `true` and `false`.

Example:
```yaml
hub:
  params:
    - name: enable-devconsole-integration
      value: "false"
```

### Dashboard

Dashboard provides configuration options for the Tekton Dashboard if the specified profile value includes the Dashboard component. (E.g. `all` on Kubernetes)

Example:

```yaml
dashboard:
  readonly: true
```

- `readonly`: If set to true, install the Dashboard in read-only mode

This is an `Optional` section.


[node-selector]:https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
[tolerations]:https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
[schedule]:https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#cron-schedule-syntax
