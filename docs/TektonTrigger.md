<!--
---
linkTitle: "TektonTrigger"
weight: 3
---
-->
# Tekton Trigger

TektonTrigger custom resource allows user to install and manage [Tekton Trigger][trigger]. 

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonTrigger CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonTrigger
metadata:
  name: trigger
spec:
  targetNamespace: tekton-pipelines
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

[trigger]:https://github.com/tektoncd/triggers
