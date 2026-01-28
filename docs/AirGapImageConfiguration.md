<!--
---
linkTitle: "Air Gap Image Configuration"
weight: 101
---
-->

# Air Gap environment (aka disconnected environment)
When we have our cluster on a air gap or proxy environment,
we need to copy the actual images into our custom registry and update image details via environment variables on the operator deployment under the container `tekton-operator-lifecycle` as follows,
This will allow us to use images from our custom registry.

## Rewrite image registry 

You can rewrite the registry host of all images managed by the operator by setting the `TEKTON_REGISTRY_OVERRIDE` environment variable on the tekton-operator-lifecycle container. This keeps the original repository path and tag/digest, and only changes the registry host.

If not set, no change is applied (default behavior).


We can rewrite the actual registry `ghcr.io` of all images by simply set the environment variable `TEKTON_REGISTRY_OVERRIDE` ad follow:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tekton-operator
  namespace: tekton-operator
spec:
  template:
    spec:
      containers:
        - name: tekton-operator-lifecycle
          env:
            # Optional: globally rewrite registry host for all images
            - name: TEKTON_REGISTRY_OVERRIDE
              value: my-internal-registry.io/my-tekton-folder
            # You can still specify per-image values; their registry host will be rewritten to the override above
            - name: IMAGE_DASHBOARD_TEKTON_DASHBOARD
              value: ghcr.io/tektoncd/dashboard:v0.48.0
```
Behavior and precedence:

- If `TEKTON_REGISTRY_OVERRIDE` is unset, images are taken from per-image env vars (if set) or from the shipped defaults.
- If `TEKTON_REGISTRY_OVERRIDE` is set, the operator rewrites the registry host for all resolved images (from per-image env vars and defaults). The repository path and tag/digest are preserved.
- There is currently no per-image opt-out when the global override is set. To exempt specific images, do not set `TEKTON_REGISTRY_OVERRIDE` and rely solely on per-image env vars.

## Rewrite image one by one

We can also rewrite images one by one using the following:

##### Sample: images as environment variable in operator deployment
```yaml
example.com/tektoncd/dashboard:v0.48.0
            - name: IMAGE_JOB_PRUNER_TKN
              value: custom-example.com/tektoncd/tkn:v0.31.0
```

### Tekton instance update

If you update an existing instance of tekton, you will need also to refresh the `TektonInstallerSets` so the new value can be taken into account.

```bash
kubectl delete tektoninstallerset <installer-set-name>
```

### List of image environment variables

#### Images supported in kubernetes

| Component            | Container/Args name                | Environment Variable                               |
|----------------------|------------------------------------|----------------------------------------------------|
| Chains               | tekton-chains-controller           | `IMAGE_CHAINS_TEKTON_CHAINS_CONTROLLER`            |
| Dashboard            | tekton-dashboard                   | `IMAGE_DASHBOARD_TEKTON_DASHBOARD`                 |
| Hub                  | tekton-hub-api                     | `IMAGE_HUB_TEKTON_HUB_API`                         |
| Hub                  | tekton-hub-db                      | `IMAGE_HUB_TEKTON_HUB_DB`                          |
| Hub                  | tekton-hub-db-migration            | `IMAGE_HUB_TEKTON_HUB_DB_MIGRATION`                |
| Hub                  | tekton-hub-ui                      | `IMAGE_HUB_TEKTON_HUB_UI`                          |
| Manual Approval Gate | manual-approval                    | `IMAGE_MAG_MANUAL_APPROVAL`                        |
| Manual Approval Gate | tekton-taskgroup-controller        | `IMAGE_MAG_TEKTON_TASKGROUP_CONTROLLER`            |
| Pipeline             | arg:entrypoint-image               | `IMAGE_PIPELINES_ARG__ENTRYPOINT_IMAGE`            |
| Pipeline             | arg:git-image                      | `IMAGE_PIPELINES_ARG__GIT_IMAGE`                   |
| Pipeline             | arg:nop-image                      | `IMAGE_PIPELINES_ARG__NOP_IMAGE`                   |
| Pipeline             | arg:shell-image                    | `IMAGE_PIPELINES_ARG__SHELL_IMAGE`                 |
| Pipeline             | arg:shell-image-win                | `IMAGE_PIPELINES_ARG__SHELL_IMAGE_WIN`             |
| Pipeline             | arg:workingdirinit-image           | `IMAGE_PIPELINES_ARG__WORKINGDIRINIT_IMAGE`        |
| Pipeline             | controller (resolvers controller)  | `IMAGE_PIPELINES_CONTROLLER`                       |
| Pipeline             | tekton-events-controller           | `IMAGE_PIPELINES_TEKTON_EVENTS_CONTROLLER`         |
| Pipeline             | tekton-pipelines-controller        | `IMAGE_PIPELINES_TEKTON_PIPELINES_CONTROLLER`      |
| Pipeline             | webhook                            | `IMAGE_PIPELINES_WEBHOOK`                          |
| Results              | api                                | `IMAGE_RESULTS_API`                                |
| Results              | postgres                           | `IMAGE_RESULTS_POSTGRES`                           |
| Results              | watcher                            | `IMAGE_RESULTS_WATCHER`                            |
| Triggers             | arg:el-image                       | `IMAGE_TRIGGERS_ARG__EL_IMAGE`                     |
| Triggers             | tekton-triggers-controller         | `IMAGE_TRIGGERS_TEKTON_TRIGGERS_CONTROLLER`        |
| Triggers             | tekton-triggers-core-interceptors  | `IMAGE_TRIGGERS_TEKTON_TRIGGERS_CORE_INTERCEPTORS` |
| Triggers             | webhook                            | `IMAGE_TRIGGERS_WEBHOOK`                           |
| Pipelines Proxy      | webhook Proxy image                | `IMAGE_PIPELINES_PROXY`                            |
| Pruner CronJob       | image used in pruner cronJob       | `IMAGE_JOB_PRUNER_TKN`                             |
| Tekton Pruner        | image used by pruner controller    | `IMAGE_PRUNER_CONTROLLER`                          |
| Tekton Pruner        | image used by pruner webhook       | `IMAGE_PRUNER_WEBHOOK`                             |
| Tekton Scheduler     | image used by scheduler controller | `IMAGE_SCHEDULER_WEBHOOK`                          |
| Tekton Scheduler     | image used by scheduler webhook       | `IMAGE_SCHEDULER_WEBHOOK`                             |


#### Images supported in OpenShift
Supports all the images listed above in kubernetes and following are specific to OpenShift

| Component             | Container/Args name               | Environment Variable                                |
|-----------------------|-----------------------------------|-----------------------------------------------------|
| Pipeline-as-code      | pac-controller                    | `IMAGE_PAC_PAC_CONTROLLER`                          |
| Pipeline-as-code      | pac-webhook                       | `IMAGE_PAC_PAC_WEBHOOK`                             |
| Pipeline-as-code      | pac-watcher                       | `IMAGE_PAC_PAC_WATCHER`                             |
| Console Plugin        | console-plugin                    | `IMAGE_PIPELINES_CONSOLE_PLUGIN`                    |
| Results               | retention-policy-agent            | `IMAGE_RESULTS_RETENTION_POLICY_AGENT`              |
| Addons                |                                   | `IMAGE_ADDONS_BUILD`                                |
| Addons                |                                   | `IMAGE_ADDONS_GENERATE`                             |
| Addons                |                                   | `IMAGE_ADDONS_GEN_ENV_FILE`                         |
| Addons                |                                   | `IMAGE_ADDONS_GIT_RUN`                              |
| Addons                |                                   | `IMAGE_ADDONS_KN`                                   |
| Addons                |                                   | `IMAGE_ADDONS_LOAD_SCRIPTS`                         |
| Addons                |                                   | `IMAGE_ADDONS_MAVEN_GENERATE`                       |
| Addons                |                                   | `IMAGE_ADDONS_MAVEN_GOALS`                          |
| Addons                |                                   | `IMAGE_ADDONS_MVN_SETTINGS`                         |
| Addons                |                                   | `IMAGE_ADDONS_OC`                                   |
| Addons                |                                   | `IMAGE_ADDONS_PARAM_BUILDER_IMAGE`                  |
| Addons                |                                   | `IMAGE_ADDONS_PARAM_GITINITIMAGE`                   |
| Addons                |                                   | `IMAGE_ADDONS_PARAM_KN_IMAGE`                       |
| Addons                |                                   | `IMAGE_ADDONS_PARAM_MAVEN_IMAGE`                    |
| Addons                |                                   | `IMAGE_ADDONS_PARAM_TKN_IMAGE`                      |
| Addons                |                                   | `IMAGE_ADDONS_PREPARE`                              |
| Addons                |                                   | `IMAGE_ADDONS_REPORT`                               |
| Addons                |                                   | `IMAGE_ADDONS_S2I_BUILD`                            |
| Addons                |                                   | `IMAGE_ADDONS_S2I_GENERATE`                         |
| Addons                |                                   | `IMAGE_ADDONS_SKOPEO_COPY`                          |
| Addons                |                                   | `IMAGE_ADDONS_SKOPEO_RESULTS`                       |
| Addons                |                                   | `IMAGE_ADDONS_TKN`                                  |
| Addons                |                                   | `IMAGE_ADDONS_TKN_CLI_SERVE`                        |
| Addons                |                                   | `IMAGE_ADDONS_TKN_CLI_SERVE_INIT_CONFIG`            |
