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

    If set to true, installs the Dashboard in read-only mode.

[dashboard]:https://github.com/tektoncd/dashboard
