<!--
---
linkTitle: "TektonDashboard"
weight: 4
---
-->
# Tekton Dashboard

TektonDashboard custom resource allows user to install and manage [Tekton Dashboard][dashboard].

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonDashboard CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonDashboard
metadata:
  name: dashboard
spec:
  targetNamespace: tekton-pipelines
  readonly: false
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.


### Properties

- `readonly` (Default: `false`)

  If set to `true`, installs the Dashboard in read-only mode.
  
  If set to `false`, the following features will be enabled on the Dashboard:
  
  - delete a pipeline
  - create a pipelinerun
  - rerun a pipelinerun
  - delete a pipelinerun
  - create a pipelineresource
  - create a taskrun
  - import resources from repository

- `external-logs`

  External URL from which to fetch logs when logs are not available in the cluster  

[dashboard]:https://github.com/tektoncd/dashboard
