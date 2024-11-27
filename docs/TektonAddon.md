<!--
---
linkTitle: "TektonAddon"
weight: 6
---
-->
# Tekton Addon

TektonAddon custom resource allows user to install resource like resolverTasks, resolverStepActions, communityResolverTasks and pipelineTemplate along with Pipelines.
It also allows user to install various Tasks in openshift-pipelines namespace.

**NOTE:** TektonAddon is currently available only for OpenShift Platform. This is roadmap to enable it for Kubernetes platform.

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonAddon CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonAddon
metadata:
  name: addon
spec:
  targetNamespace: openshift-pipelines
  params:
  - name: pipelineTemplates
    value: "true"
  - name: resolverTasks
    value: "true"
  - name: resolverStepActions
    value: "true"
  - name: communityResolverTasks
    value: "true"
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

### Params

params provide a way to enable/disable the installation of resources.
Available params are
- `pipelineTemplates` (Default: `true`)
- `resolverTasks` (Default: `true`)
- `resolverStepActions` (Default: `true`)
- `communityResolverTasks` (Default: `true`)

User can disable the installation of resources by changing the value to `false`.

- Pipelines templates uses tasks from `openshift-pipelines`. Therefore, to install pipelineTemplates, resolverTasks must be set to `true`
