<!--
---
linkTitle: "TektonPruner"
weight: 30
---
-->
# Tekton Pruner

TektonPruner custom resource allows user to install and manage [Event Based Tekton Pruner][pruner].

It is recommended to install the component through [TektonConfig](./TektonConfig.md).

**NOTE**: Job based pruner **MUST** be disabled for the new event based pruner to be enabled.

- TektonPruner CR is as below

    - On Kubernetes, TektonPruner CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonPruner
    metadata:
      name: pruner
    spec:
      disabled: false
      targetNamespace: tekton-pipelines
    ```

    - On OpenShift, TektonPruner CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonPruner
    metadata:
      name: pruner
    spec:
      disabled: false
      targetNamespace: openshift-pipelines
    ```

- Check the status of installation using following command:

    ```sh
    kubectl get tektonpruners.operator.tekton.dev
    ```

## Pruner Config

Right now, event based pruner config just allows to either enable or disable the new pruner.


### Properties (Mandatory)

 - `targetNamespace`

    Setting this field to provide the namespace in which you want to install the pruner component.

- `disabled` : if the value set as `true`, pruner feature will be disabled (default: `true`)

Rest of the configurations as defined [here][pruner-config] are currently managed with the configmap [`tekton-pruner-default-spec`](https://github.com/openshift-pipelines/tektoncd-pruner/blob/main/config/600-tekton-pruner-default-spec.yaml).



[pruner]:https://github.com/openshift-pipelines/tektoncd-pruner
[pruner-config]:https://github.com/openshift-pipelines/tektoncd-pruner/blob/main/docs/tutorials/README.md