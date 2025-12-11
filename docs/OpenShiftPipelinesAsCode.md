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
      additionalPACControllers:
        controllername:
          enable: true
          configMapName:
          secretName:
          settings:
      enable: true
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
        error-log-snippet-number-of-lines: "3"
        enable-cancel-in-progress-on-pull-requests: "false"
        enable-cancel-in-progress-on-push: "false"
        hub-catalog-name: tekton
        hub-url: https://api.hub.tekton.dev/v1
        require-ok-to-test-sha: "false"
        skip-push-event-for-pr-commits: "true"
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

### Properties (Optional)

The fields have default values so even if the user has not passed them in CR, operator will add them. The user can later change
them as per their need.

Details of the field can be found in [OpenShift Pipelines As Code Settings][pac-config]

#### Remote Hub Catalogs

Pipelines as Code supports configuring remote hub catalogs to fetch tasks and pipelines. You can configure custom catalogs using the `catalog-{INDEX}-*` settings pattern.

Each custom catalog requires the following four fields:

| Field | Description | Required |
|-------|-------------|----------|
| `catalog-{INDEX}-id` | Unique identifier for the catalog. Users reference tasks from this catalog using this ID as prefix (e.g., `custom://task-name`) | Yes |
| `catalog-{INDEX}-name` | Name of the catalog within the hub | Yes |
| `catalog-{INDEX}-url` | Full URL endpoint of the hub API | Yes |
| `catalog-{INDEX}-type` | Type of hub: `artifacthub` (for Artifact Hub) or `tektonhub` (for Tekton Hub) | Yes |

Where `{INDEX}` is a number that can be incremented to add multiple catalogs (e.g., `1`, `2`, `3`).

##### Example: Configuring a Custom Hub Catalog

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: OpenShiftPipelinesAsCode
metadata:
  name: pipelines-as-code
spec:
  enable: true
  settings:
    catalog-1-id: "custom"
    catalog-1-name: "tekton"
    catalog-1-url: "https://api.custom.hub/v1"
    catalog-1-type: "tektonhub"
  targetNamespace: openshift-pipelines
```

##### Example: Configuring Multiple Hub Catalogs

You can configure multiple hub catalogs by incrementing the `catalog-{INDEX}` number:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: OpenShiftPipelinesAsCode
metadata:
  name: pipelines-as-code
spec:
  enable: true
  settings:
    catalog-1-id: "custom"
    catalog-1-name: "tekton"
    catalog-1-url: "https://api.custom.hub/v1"
    catalog-1-type: "tektonhub"
    catalog-2-id: "artifact"
    catalog-2-name: "tekton-catalog-tasks"
    catalog-2-url: "https://artifacthub.io"
    catalog-2-type: "artifacthub"
  targetNamespace: openshift-pipelines
```

##### Referencing Tasks from Custom Catalogs

Once configured, users can reference tasks from custom catalogs by adding a prefix matching the catalog ID:

```yaml
pipelinesascode.tekton.dev/task: "custom://git-clone"
pipelinesascode.tekton.dev/task: "artifact://buildah"
```

> **Note:** Pipelines-as-Code will not try to fallback to the default or another custom hub if the task referenced is not found (the Pull Request will be set as failed).

For more details, see the [Pipelines-as-Code Remote Hub Catalogs documentation](https://pipelinesascode.com/docs/install/settings/#remote-hub-catalogs).

#### Additional Pipelines As Code Controller (Optional)

If users want to deploy additional Pipelines As Code controller on their cluster along with default Pipelines As Code 
controller then they need to provide the `additionalPACControllers` field in the `pipelinesAsCode` section.

Example:

```yaml
pipelinesAsCode:
  additionalPACControllers: # can provide a list of controllers
    <controllername>: 
      enable:
      configMapName:
      secretName:
      settings:
```

- `controllername` is the unique name of the new controller, should not be more than 25 characters and should follow k8s naming rules.

- `enable` is optional with default value to true. You can use this field to disable the additional PAC controller
   without removing the details from the CR.

- `configMapName` is optional and is to provide the ConfigMap name of additional PAC Controller. If user doesn't 
   provide any value then Operator will add controllerName + `-pipelines-as-code-configmap` as default value. If user 
   provides configMap name as `pipelines-as-code` for additional Pipelines As Code controller, then operator will not create
   the configMap and the default `pipeline-as-code` configMap will be used with default settings.

- `secretName` is optional and is to provide the secret name of additional PAC Controller. If user does not provide any 
   value then operator will add controllerName + `-pipelines-as-code-secret` as default value to be added to deployment env.

- `settings` is optional and used to set the settings in the configMap of additional PAC Controller. For the fields whose
   are not provided, default value will be used. You can check them [here](https://pipelinesascode.com/docs/install/settings/#pipelines-as-code-configuration-settings). 
   Also, if configmap name is provided as `pipelines-as-code` then these settings will not be taken.

> **NOTE:** Users can deploy multiple additional PAC Controller by providing multiple entries in `additionalPACControllers` field.

Example:

```yaml
pipelinesAsCode:
  additionalPACControllers:
    firstcontroller:
      enable: true
    secondcontroller:
      enable: true
      configMapName: second-config
      secretName: second-secret
```

[PipelinesAsCode]:https://github.com/openshift-pipelines/pipelines-as-code
[pac-config]:https://pipelinesascode.com/docs/install/settings/#pipelines-as-code-configuration-settings
