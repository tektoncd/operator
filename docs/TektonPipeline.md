# Tekton Pipeline

TektonPipeline custom resource allows user to install and manage [Tekton Pipeline][Pipeline].

It is recommended to install the components through [TektonConfig](./TektonConfig.md).

The TektonPipeline CR is as below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonPipeline
metadata:
  name: pipeline
spec:
  targetNamespace: tekton-pipelines
  disable-affinity-assistant: false
  disable-creds-init: false
  disable-home-env-overwrite: true
  disable-working-directory-overwrite: true
  enable-api-fields: stable
  enable-custom-tasks: false
  enable-tekton-oci-bundles: false
  require-git-ssh-secret-known-hosts: false
  running-in-environment-with-injected-sidecars: true
  scope-when-expressions-to-task: false
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

### Properties
 This fields have default values so even if user have not passed them in CR, operator will add them. User can later change 
 them as per their need.

- `disable-affinity-assistant` (Default: `false`)
  
    Setting this flag to "true" will prevent Tekton to create an Affinity Assistant for every TaskRun sharing a PVC workspace. The default behaviour is for Tekton to create Affinity Assistants. 
  
    See more in the workspace documentation about [Affinity Assistant](https://github.com/tektoncd/pipeline/blob/main/docs/workspaces.md#affinity-assistant-and-specifying-workspace-order-in-a-pipeline)
   or more info [here](https://github.com/tektoncd/pipeline/pull/2630).
  

- `disable-home-env-overwrite` (Default: `true`)
  
    Setting this flag to "false" will allow Tekton to override your Task container's $HOME environment variable.
    
    See more info [here](https://github.com/tektoncd/pipeline/issues/2013).


- `disable-working-directory-overwrite` (Default: `true`)
    
    Setting this flag to "false" will allow Tekton to override your Task container's working directory. 
    
    See more info [here](https://github.com/tektoncd/pipeline/issues/1836).


- `disable-creds-init` (Default: `false`)
    
    Setting this flag to "true" will prevent Tekton scanning attached service accounts and injecting any credentials it 
    finds into your Steps.
    
    The default behaviour currently is for Tekton to search service accounts for secrets matching a specified format and 
    automatically mount those into your Steps. 
    
    Note: setting this to "true" will prevent PipelineResources from working. See more info [here](https://github.com/tektoncd/pipeline/issues/2791).


- `running-in-environment-with-injected-sidecars` (Default: `true`)
  
    This option should be set to false when Pipelines is running in a cluster that does not use injected sidecars such 
    as Istio. Setting it to false should decrease the time it takes for a TaskRun to start running. For clusters that use 
    injected sidecars, setting this option to false can lead to unexpected behavior.
  
    See more info [here](https://github.com/tektoncd/pipeline/issues/2080).


-  `require-git-ssh-secret-known-hosts` (Default: `false`)

   Setting this flag to "true" will require that any Git SSH Secret offered to Tekton must have known_hosts included.

   See more info [here](https://github.com/tektoncd/pipeline/issues/2981).


- `enable-tekton-oci-bundles` (Default: `false`)
  
    Setting this flag to "true" enables the use of Tekton OCI bundle. This is an experimental feature and thus should 
    still be considered an alpha feature.
  
    
- `enable-custom-tasks` (Default: `false`)
  
    Setting this flag to "true" enables the use of custom tasks from within pipelines. This is an experimental feature 
    and thus should still be considered an alpha feature.


- `enable-api-fields` (Default: `stable`)

    Setting this flag will determine which gated features are enabled. Acceptable values are "stable" or "alpha".


- `scope-when-expressions-to-task` (Default: `false`)

  Setting this flag to "true" scopes when expressions to guard a Task only instead of a Task and its dependent Tasks.

### Optional Properties
This fields doesn't have default values so will be considered only if user passes them. By default Operator won't add 
this fields CR and won't configure for pipelines. 

The Default values for this fields are already set in pipelines are not set by Operator. If user passes some values then
those will be set for the particular field.

- `default-timeout-minutes`
  
    default-timeout-minutes contains the default number of minutes to use for TaskRun and PipelineRun, if none is specified.
    
    `default-timeout-minutes: "60"  # 60 minutes`
  

- `default-service-account`
  
    default-service-account contains the default service account name to use for TaskRun and PipelineRun, if none is specified.
    
    `default-service-account: "default"`


- `default-managed-by-label-value`
  
    default-managed-by-label-value contains the default value given to the "app.kubernetes.io/managed-by" label applied 
    to all Pods created for TaskRuns. If a user's requested TaskRun specifies another value for this label, the user's
    request supersedes.
  
    `default-managed-by-label-value: "tekton-pipelines"`


- `default-pod-template`
  
    default-pod-template contains the default pod template to use TaskRun and PipelineRun, if none is specified. If a
    pod template is specified, the default pod template is ignored.
  

- `default-cloud-events-sink`
  
    default-cloud-events-sink contains the default CloudEvents sink to be used for TaskRun and PipelineRun, when no sink
    is specified. Note that right now it is still not possible to set a PipelineRun or TaskRun specific sink, so the 
    default is the only option available. If no sink is specified, no CloudEvent is generated


- `default-task-run-workspace-binding`
  
    default-task-run-workspace-binding contains the default workspace configuration provided for any Workspaces that a
    Task declares but that a TaskRun does not explicitly provide.

[Pipeline]:https://github.com/tektoncd/pipeline
