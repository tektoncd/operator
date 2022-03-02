<!--
---
linkTitle: "TektonChains"
weight: 9
---
-->
# Tekton Chains

TektonChains custom resource allows user to install and manage [Tekton Chains][chains]. 

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonChains CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonChains
metadata:
  name: chains
spec:
  targetNamespace: tekton-pipelines
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

[chains]:https://github.com/tektoncd/chains
