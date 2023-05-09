<!--
---
linkTitle: "TektonPipeline"
weight: 2
---
-->
# Tekton Pipeline

TektonPipeline custom resource allows user to install and manage [Tekton Pipeline][Pipeline].

It is recommended to install the component through [TektonConfig](./TektonConfig.md).

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
  metrics.pipelinerun.duration-type: histogram
  metrics.pipelinerun.level: pipeline
  metrics.taskrun.duration-type: histogram
  metrics.taskrun.level: task
  require-git-ssh-secret-known-hosts: false
  running-in-environment-with-injected-sidecars: true
  scope-when-expressions-to-task: false
  trusted-resources-verification-no-match-policy: ignore
  performance:
    disable-ha: false
    buckets: 1
    threads-per-controller: 2
    kube-api-qps: 5.0
    kube-api-burst: 10
```
You can install this component using [TektonConfig](./TektonConfig.md) by choosing appropriate `profile`.

### Properties
This fields have default values so even if user have not passed them in CR, operator will add them. User can later change
them as per their need.

- `disable-affinity-assistant` (Default: `false`)

    Setting this flag to "true" will prevent Tekton to create an Affinity Assistant for every TaskRun sharing a PVC workspace. The default behaviour is for Tekton to create Affinity Assistants.

    See more in the workspace documentation about [Affinity Assistant](https://github.com/tektoncd/pipeline/blob/main/docs/workspaces.md#affinity-assistant-and-specifying-workspace-order-in-a-pipeline)
    or more info [here](https://github.com/tektoncd/pipeline/pull/2630).

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

- `trusted-resources-verification-no-match-policy` (Default: `ignore`)

    Trusted Resources is a feature which can be used to sign Tekton Resources and verify them. Details of design can be found at [TEP–0091](https://github.com/tektoncd/community/blob/main/teps/0091-trusted-resources.md). This feature is under alpha version and support v1beta1 version of Task and Pipeline. To know more about this visit [pipelines documentation](https://tekton.dev/docs/pipelines/trusted-resources/)

### Metrics Properties
These fields have default values so even if user have not passed them in CR, operator will add them and override the values
configure in pipelines.

- `metrics.pipelinerun.duration-type` (Default: `histogram`)

    Setting this flag will determine the duration type - gauge or histogram.

- `metrics.pipelinerun.level` (Default: `pipeline`)

    Setting this flag will determine the level of pipelinerun metrics.

- `metrics.taskrun.duration-type` (Default: `histogram`)

    Setting this flag will determine the duration type - gauge or histogram.

- `metrics.taskrun.level` (Default: `task`)

    Setting this flag will determine the level of taskrun metrics.



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

### Performance Properties
```yaml
spec:
  # omitted other fields ...
  performance:
    disable-ha: false
    buckets: 1
    threads-per-controller: 2
    kube-api-qps: 5.0
    kube-api-burst: 10
```
These fields are optional and there is no default values. If user passes them, operator will include most of fields into the deployment `tekton-pipelines-controller` under the container `tekton-pipelines-controller` as arguments(duplicate name? No, container and deployment has the same name), otherwise pipelines controller's default values will be considered. and `buckets` field is updated into `config-leader-election` config-map under the namespace `tekton-pipelines`.

A high level descriptions are given here. To get the detailed information please visit pipelines documentation, [High Availability Support](https://tekton.dev/docs/pipelines/enabling-ha/), and [Performance Configuration](https://tekton.dev/docs/pipelines/tekton-controller-performance-configuration/)


* `disable-ha` - enable or disable ha feature, defaults in pipelines controller is `disable-ha=false`
* `buckets` - buckets is the number of buckets used to partition key space of each reconciler. If this number is M and the replica number of the controller is N, the N replicas will compete for the M buckets. The owner of a bucket will take care of the reconciling for the keys partitioned into that bucket. The maximum value of `buckets` at this time is `10`. default value in pipeline controller is `1`
* `threads-per-controller` - is the number of threads(aka worker) to use when processing the pipelines controller's workqueue, default value in pipelines controller is `2`
* `kube-api-qps` - QPS indicates the maximum QPS to the cluster master from the REST client, default value in pipeline controller is `5.0`
* `kube-api-burst` - maximum burst for throttle, default value in pipeline controller is `10`

> #### Note:
>
> * Pipelines controller deployments `replicas` will not be handled by operator. User has to change the replicas as follows,
>   ```
>   kubectl --namespace tekton-pipelines scale deployment tekton-pipelines-controller --replicas=3
>   ```
>
> * `kube-api-qps` and `kube-api-burst` will be multiplied by 2 in pipelines controller. To get the detailed information visit [Performance Configuration](https://tekton.dev/docs/pipelines/tekton-controller-performance-configuration/) guide
> * if you modify or remove any of the performance properties, `tekton-pipelines-controller` deployment and `config-leader-election` config-map (if `buckets` changed) will be updated, and `tekton-pipelines-controller` pods will be recreated
