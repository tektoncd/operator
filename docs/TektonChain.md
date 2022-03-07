<!--
---
linkTitle: "TektonChain"
weight: 9
---
-->
# Tekton Chain

TektonChain custom resource allows user to install and manage [Tekton Chains][chains].

TektonChain is an optional component and currently cannot be installed through TektonConfig. It has to be installed separately.

To install TektonChain on your cluster follow steps as given below:

- Make sure Tekton Pipelines is installed on your cluster, using the Operator.

- Create the TektonChain CR.

    - On Kubernetes, TektonChain CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonChain
    metadata:
      name: chain
    spec:
      targetNamespace: tekton-pipelines
    ```

    - On OpenShift, TektonChain CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonChain
    metadata:
      name: chain
    spec:
      targetNamespace: openshift-pipelines
    ```

- Check the status of installation using following command:

    ```sh
    kubectl get tektonchains.operator.tekton.dev
    ```

[chains]:https://github.com/tektoncd/chains
