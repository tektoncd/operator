<!--
---
linkTitle: "TektonAddon"
weight: 6
---
-->
# Tekton Addon

TektonAddon custom resource allows user to install resource like clusterTasks and pipelineTemplate along with Pipelines. 

**NOTE:** TektonAddon is currently available only for OpenShift Platform. This is roadmap to enable it for Kubernetes platform.

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonAddon CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonAddon
metadata:
  name: addon
spec:
  targetNamespace: openshift-pipelines
  params:
  - name: clusterTasks
    value: "true"
  - name: pipelineTemplates
    value: "true"
  enablePipelinesAsCode: true
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

### Params

params provide a way to enable/disable the installation of resources.
Available params are

- `clusterTasks` (Default: `true`)
- `pipelineTemplates` (Default: `true`)

User can disable the installation of resources by changing the value to `false`.

Pipelines templates uses clustertasks in them so to install pipelineTemplates, clusterTasks must be `true`.

### PipelinesAsCode

`enablePipelinesAsCode` field is provided in spec to enable/disable PipelinesAsCode installation on the cluster.

User can disable the installation of PipelinesAsCode by changing the value to `false`. By default, it is true.
