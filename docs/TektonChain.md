<!--
---
linkTitle: "TektonChain"
weight: 9
---
-->
# Tekton Chain

TektonChain custom resource allows user to install and manage [Tekton Chains][chains].

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonChain CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonChain
metadata:
  name: chain
spec:
  targetNamespace: tekton-pipelines
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

[chains]:https://github.com/tektoncd/chains
