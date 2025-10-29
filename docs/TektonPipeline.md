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
  await-sidecar-readiness: true
  coschedule: workspaces
  disable-affinity-assistant: false
  disable-creds-init: false
  disable-home-env-overwrite: true
  disable-working-directory-overwrite: true
  disable-inline-spec: "taskrun,pipelinerun,pipeline"
  enable-api-fields: beta
  enable-bundles-resolver: true
  enable-cel-in-whenexpression: false
  enable-cluster-resolver: true
  enable-custom-tasks: true
  enable-git-resolver: true
  enable-hub-resolver: true
  enable-param-enum: false
  enable-provenance-in-status: true
  enable-step-actions: false
  enforce-nonfalsifiability: none
  keep-pod-on-cancel: false
  max-result-size: 4096
  metrics.count.enable-reason: false
  metrics.pipelinerun.duration-type: histogram
  metrics.pipelinerun.level: pipeline
  metrics.taskrun.duration-type: histogram
  metrics.taskrun.level: task
  # Tracing configuration (optional)
  # traces.enabled: true
  # traces.endpoint: "http://jaeger-collector.jaeger.svc.cluster.local:4318/v1/traces"
  # traces.credentialsSecret: ""
  require-git-ssh-secret-known-hosts: false
  results-from: termination-message
  running-in-environment-with-injected-sidecars: true
  send-cloudevents-for-runs: false
  set-security-context: false
  trusted-resources-verification-no-match-policy: ignore
  performance:
    disable-ha: false
    buckets: 1
    replicas: 1
    threads-per-controller: 2
    kube-api-qps: 5.0
    kube-api-burst: 10
    statefulset-ordinals: false
  options:
    disabled: false
    configMaps: {}
    deployments: {}
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

- `await-sidecar-readiness` (Default: `true`)

    Setting this flag to "false" to allow the Tekton controller to start a TasksRun's first step immediately without 
    waiting for sidecar containers to be running first.

    Note: setting this flag to "false" will mean the running-in-environment-with-injected-sidecars flag has no effect.

- `coschedule` (Default: `workspaces`)

    This flag determines how PipelineRun Pods are scheduled with Affinity Assistant. Acceptable values are 
    "workspaces" (default), "pipelineruns", "isolate-pipelinerun", or "disabled"

- `running-in-environment-with-injected-sidecars` (Default: `true`)

    This option should be set to false when Pipelines is running in a cluster that does not use injected sidecars such
as Istio. Setting it to false should decrease the time it takes for a TaskRun to start running. For clusters that use
injected sidecars, setting this option to false can lead to unexpected behavior.

    See more info [here](https://github.com/tektoncd/pipeline/issues/2080).


-  `require-git-ssh-secret-known-hosts` (Default: `false`)

    Setting this flag to "true" will require that any Git SSH Secret offered to Tekton must have known_hosts included.

    See more info [here](https://github.com/tektoncd/pipeline/issues/2981).


- `enable-custom-tasks` (Default: `false`)

    Setting this flag to "true" enables the use of custom tasks from within pipelines. This is an experimental feature
and thus should still be considered an alpha feature.


- `enable-api-fields` (Default: `stable`)

    Setting this flag will determine which gated features are enabled. Acceptable values are "stable" or "alpha".

- `results-from` (Default: `termination-message`)

    This feature is to use the container's termination message to fetch results from. Set it to "sidecar-logs" to 
    enable use of a results sidecar logs to extract results instead of termination message.

- `max-result-size` (Default: `4096`)

    This feature is to configure the size of the task results if using `sidecar-logs`. The default value if `4096` and
    maximum value can be `1572863`.

- `enable-provenance-in-status` (Default: `true`)

    This feature is to enable populating the provenance field in TaskRun and PipelineRun status. The provenance field 
    contains metadata about resources used in the TaskRun/PipelineRun such as the source from where a remote 
    Task/Pipeline definition was fetched. To disable populating this field, set this flag to "false".

- `set-security-context` (Default: `false`)

    Setting this flag to "true" to set a security context for containers injected by Tekton that will allow TaskRun pods 
    to run in namespaces with restricted pod security admission

- `keep-pod-on-cancel` (Default: `false`)

    Setting this flag to "true" will not delete the pod associated with cancelled taskrun.

- `enforce-nonfalsifiability` (Default: `none`)

    Setting this flag to "spire" to enable integration with `SPIRE`.

- `enable-param-enum` (Default: `false`)

    Setting this flag to "true" will enable params of type `Enum`

- `enable-step-actions` (Default: `false`)

    Setting this flag to "true" will enable specifying `StepAction` in a `Step`. A `StepAction` is the reusable and 
    scriptable unit of work that is performed by a `Step`

- `enable-cel-in-whenexpression` (Default: `false`)

    Setting this flag to "true" will enable using CEL in when expressions.

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

- `metrics.count.enable-reason` (Default: `false`)

    Setting this flag to "true" will include reason label on count metrics.

### Tracing Properties

Tekton Pipelines supports distributed tracing using OpenTelemetry. These fields allow you to configure tracing to send trace data to observability backends like Jaeger, Zipkin, or other OTLP-compatible collectors.

- `traces.enabled` (Optional)

    Setting this flag to "true" enables distributed tracing for Tekton Pipelines. When enabled, the pipeline controller will export trace spans for pipeline and task reconciliation operations.

    ```yaml
    spec:
      traces.enabled: true
    ```

- `traces.endpoint` (Optional)

    The URL of the OpenTelemetry trace collector endpoint. Tekton Pipeline exports traces using the OTLP (OpenTelemetry Protocol) format over HTTP.

    Supported endpoints:
    - **Jaeger with OTLP**: Use port 4318 for OTLP HTTP (e.g., `http://jaeger-collector.jaeger.svc.cluster.local:4318/v1/traces`)
    - **OpenTelemetry Collector**: OTLP HTTP endpoint (e.g., `http://otel-collector.observability.svc.cluster.local:4318/v1/traces`)
    - **Other OTLP-compatible backends**: Any trace backend that supports OTLP HTTP protocol

    ```yaml
    spec:
      traces.enabled: true
      traces.endpoint: "http://jaeger-collector.jaeger.svc.cluster.local:4318/v1/traces"
    ```

- `traces.credentialsSecret` (Optional)

    The name of a Kubernetes secret containing credentials for authenticating with the tracing endpoint. This is useful when your tracing backend requires authentication.

    ```yaml
    spec:
      traces.enabled: true
      traces.endpoint: "https://secure-jaeger-collector.example.com/api/traces"
      traces.credentialsSecret: "jaeger-auth-secret"
    ```

**Example: Complete Tracing Configuration**

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonPipeline
metadata:
  name: pipeline
spec:
  targetNamespace: tekton-pipelines
  # ... other configuration ...

  # Enable tracing with OpenTelemetry Collector
  traces.enabled: true
  traces.endpoint: "http://otel-collector.observability.svc.cluster.local:4318/v1/traces"
```

**Example: Tracing with Jaeger**

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonPipeline
metadata:
  name: pipeline
spec:
  targetNamespace: tekton-pipelines
  # ... other configuration ...

  # Enable tracing with Jaeger (using OTLP endpoint)
  traces.enabled: true
  traces.endpoint: "http://jaeger-collector.jaeger.svc.cluster.local:4318/v1/traces"
```

For more information about distributed tracing in Tekton Pipelines, see the [TEP-0124: Distributed Tracing for Tasks and Pipelines](https://github.com/tektoncd/community/blob/main/teps/0124-distributed-tracing-for-tasks-and-pipelines.md).

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

- `disable-inline-spec` (Default: ``)

    Inline specifications can be disabled for specific resources only. To achieve that, set the disable-inline-spec flag to a comma-separated list of the desired resources. Valid values are `pipeline`, `pipelinerun` and `taskrun`.

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


- `default-resolver-type`

  default-resolver-type contains the resolver type to be used as default resolver.

[Pipeline]:https://github.com/tektoncd/pipeline

### Performance Properties
```yaml
spec:
  # omitted other fields ...
  performance:
    disable-ha: false
    buckets: 1
    replicas: 1
    threads-per-controller: 2
    kube-api-qps: 5.0
    kube-api-burst: 10
    statefulset-ordinals: false
```
These fields are optional and there is no default values. If user passes them, operator will include most of fields into the deployment `tekton-pipelines-controller` under the container `tekton-pipelines-controller` as arguments(duplicate name? No, container and deployment has the same name), otherwise pipelines controller's default values will be considered. and `buckets` field is updated into `config-leader-election` config-map under the namespace `tekton-pipelines`.

A high level descriptions are given here. To get the detailed information please visit pipelines documentation, [High Availability Support](https://tekton.dev/docs/pipelines/enabling-ha/), and [Performance Configuration](https://tekton.dev/docs/pipelines/tekton-controller-performance-configuration/)


* `disable-ha` - enable or disable ha feature, defaults in pipelines controller is `disable-ha=false`
* `buckets` - buckets is the number of buckets used to partition key space of each reconciler. If this number is M and the replica number of the controller is N, the N replicas will compete for the M buckets. The owner of a bucket will take care of the reconciling for the keys partitioned into that bucket. The maximum value of `buckets` at this time is `10`. default value in pipeline controller is `1`
* `replicas` - pipelines controller deployment replicas count
* `threads-per-controller` - is the number of threads(aka worker) to use when processing the pipelines controller's workqueue, default value in pipelines controller is `2`
* `kube-api-qps` - QPS indicates the maximum QPS to the cluster master from the REST client, default value in pipeline controller is `5.0`
* `kube-api-burst` - maximum burst for throttle, default value in pipeline controller is `10`
* `statefulset-ordinals` - enables StatefulSet Ordinals mode for the Tekton Pipelines Controller. When set to true, the Pipelines Controller is deployed as a StatefulSet, allowing for multiple replicas to be configured with a load-balancing mode. This ensures that the load is evenly distributed across replicas, and the number of buckets is enforced to match the number of replicas.
Moreover, There are two mechanisms available for scaling for scaling Pipelines Controller horizontally: 
- Using leader election, which allows for failover, but can result in hot-spotting.
- Using StatefulSet ordinals, which doesn't allow for failover, but guarantees load is evenly spread across replicas.


> #### Note:
> * `kube-api-qps` and `kube-api-burst` will be multiplied by 2 in pipelines controller. To get the detailed information visit [Performance Configuration](https://tekton.dev/docs/pipelines/tekton-controller-performance-configuration/) guide
> * if you modify or remove any of the performance properties, `tekton-pipelines-controller` deployment and `config-leader-election` config-map (if `buckets` changed) will be updated, and `tekton-pipelines-controller` pods will be recreated
