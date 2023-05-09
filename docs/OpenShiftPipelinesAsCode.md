<!--
---
linkTitle: "OpenShiftPipelinesAsCode"
weight: 11
---
-->
# OpenShiftPipelinesAsCode

OpenShiftPipelinesAsCode custom resource allows the user to install and manage [OpenShift PipelinesAsCode](PipelinesAsCode).

It is recommended that you install OpenShiftPipelinesAsCode through [TektonConfig](./TektonConfig.md).

- OpenShiftPipelinesAsCode CR is as below

    - On OpenShift, OpenShiftPipelinesAsCode CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: OpenShiftPipelinesAsCode
    metadata:
      name: pipelines-as-code
    spec:
      settings:
        application-name: Pipelines as Code CI
        auto-configure-new-github-repo: "false"
        bitbucket-cloud-check-source-ip: "true"
        custom-console-name: ""
        custom-console-url: ""
        custom-console-url-pr-details: ""
        custom-console-url-pr-tasklog: ""
        error-detection-from-container-logs: "false"
        error-detection-max-number-of-lines: "50"
        error-detection-simple-regexp: ^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([
          ]*)?(?P<error>.*)
        error-log-snippet: "true"
        hub-catalog-name: tekton
        hub-url: https://api.hub.tekton.dev/v1
        remote-tasks: "true"
        secret-auto-create: "true"
        secret-github-app-token-scoped: "true"
      targetNamespace: openshift-pipelines
    ```

- Check the status of the installation using following command:

    ```sh
    kubectl get openshiftpipelinesascodes.operator.tekton.dev
    ```

## PipelinesAsCode Config 

The recommended way to update the OpenShiftPipelinesAsCode CR is using [TektonConfig](./TektonConfig.md) CR.

### Properties (Mandatory)

 - `targetNamespace`

    Set this field to provide the namespace in which you want to install the PipelinesAsCode component.

### Properties

The fields have default values so even if the user has not passed them in CR, operator will add them. The user can later change
them as per their need.

Details of the field can be found in [OpenShift Pipelines As Code Settings][pac-config]

[PipelinesAsCode]:https://github.com/openshift-pipelines/pipelines-as-code
[pac-config]:https://pipelinesascode.com/docs/install/settings/#pipelines-as-code-configuration-settings
