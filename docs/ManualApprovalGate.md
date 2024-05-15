<!--
---
linkTitle: "ManualApprovalGate"
weight: 9
---
-->
# Manual Approval Gate

ManualApprovalGate custom resource allows user to install and manage [Manual Approval Gate][manual-approval-gate].

- ManualApprovalGate CR is as below

    - On Kubernetes, ManualApprovalGate CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: ManualApprovalGate
    metadata:
      name: manual-approval-gate
    spec:
      targetNamespace: tekton-pipelines
    ```

    - On OpenShift, ManualApprovalGate CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: ManualApprovalGate
    metadata:
      name: manual-approval-gate
    spec:
      targetNamespace: openshift-pipelines
    ```

- Check the status of installation using following command:

    ```sh
    kubectl get get manualapprovalgates.operator.tekton.dev
    NAME                   VERSION   READY   REASON
    manual-approval-gate   v0.2.0    True
    ```

[manual-approval-gate]:https://github.com/openshift-pipelines/manual-approval-gate
