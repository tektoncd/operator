<!--
---
linkTitle: "TektonConfig"
weight: 1
---
-->
# Tekton Config

TektonConfig custom resource is the top most component of the operator which allows user to install and customize all other
components at a single place.

Operator provides support for installing and managing following operator components through `TektonConfig`:

- [TektonPipeline](./TektonPipeline.md)
- [TektonTrigger](./TektonTrigger.md)

Other than the above components depending on the platform operator also provides support for

- On both Kubernetes and OpenShift
  - [TektonChain](./TektonChain.md)
  - [TektonResult](./TektonResult.md)
- On Kubernetes
  - [TektonDashboard](./TektonDashboard.md)
- On OpenShift
  - [TektonAddon](./TektonAddon.md)
  - [OpenShiftPipelinesAsCode](./OpenShiftPipelinesAsCode.md)
- When scheduler multi-cluster is enabled with Hub role (both Kubernetes and OpenShift)
  - [TektonMulticlusterProxyAAE](./TektonMulticlusterProxyAAE.md)

The TektonConfig CR provides the following features

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  targetNamespace: tekton-pipelines
  targetNamespaceMetadata:
    labels: {}
    annotations: {}
  profile: all
  config:
    nodeSelector: <>
    tolerations: []
    priorityClassName: system-cluster-critical
  chain:
    disabled: false
  pipeline:
    await-sidecar-readiness: true
    coschedule: workspaces
    disable-creds-init: false
    disable-home-env-overwrite: true
    disable-working-directory-overwrite: true
    disable-inline-spec: "pipeline,pipelinerun,taskrun"
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
    options:
      disabled: false
      configMaps: {}
      deployments: {}
      webhookConfigurationOptions: {}
  pruner:
    disabled: false
    schedule: "0 8 * * *"
    resources:
      - taskrun
      - pipelinerun
    keep: 3
    # keep-since: 1440
    # NOTE: you can use either "keep" or "keep-since", not both
    prune-per-resource: true
  hub:
    params:
      - name: enable-devconsole-integration
        value: "true"
    options:
      disabled: false
      configMaps: {}
      deployments: {}
      webhookConfigurationOptions: {}
  dashboard:
    readonly: true
    options:
      disabled: false
      configMaps: {}
      deployments: {}
      webhookConfigurationOptions: {}
  result:
    disabled: false
    is_external_db: false
    options: {}
    performance:
      disable-ha: false
      buckets: 1
      replicas: 1
  trigger:
    disabled: false
platforms:
    openshift:
      pipelinesAsCode:
        additionalPACControllers:
          <controllerName>:
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
          error-detection-simple-regexp:
            ^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([
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
      options:
        disabled: false
        configMaps: {}
        deployments: {}
        webhookConfigurationOptions: {}
```

Look for the particular section to understand a particular field in the spec.

### Target Namespace

This allows user to choose a namespace to install the Tekton Components such as pipelines, triggers.

By default, namespace would be `tekton-pipelines` for Kubernetes and `openshift-pipelines` for OpenShift.

**Note:** Namespace `openshift-operators` is not allowed in `OpenShift` as a `targetNamespace`.

### Target Namespace Metadata

`targetNamespaceMetadata` allows user to add their custom `labels` and `annotations` to the target namespace via TektonConfig CR.

### Profile

This allows user to choose which all components to install on the cluster.
There are 3 profiles available:

- `all`: This profile will install all components (TektonPipeline, TektonTrigger, TektonResult and TektonChain)
- `basic`: This profile will install only TektonPipeline, TektonTrigger, TektonResult and TektonChain component
- `lite`: This profile will install only TektonPipeline component

On Kubernetes, `all` profile will install `TektonDashboard` and on OpenShift `TektonAddon` will be installed.

### Config

Config provides fields to configure deployments created by the Operator.
This provides following fields:

- [`nodeSelector`][node-selector]
- [`tolerations`][tolerations]
- [`priorityClassName`][priorityClassName]

User can pass the required fields and this would be passed to all Operator components which will get added in all
deployments created by Operator.

Example:

```yaml
config:
  nodeSelector:
    key: value
  tolerations:
    - key: "key1"
      operator: "Equal"
      value: "value1"
      effect: "NoSchedule"
  priorityClassName: system-node-critical
```

This is an `Optional` section.

**NOTE**: If `spec.config.priorityClassName` is used, then the required [`priorityClass`][priorityClass] is
expected to be created by the user to get the Tekton resources pods in running state

### Pipeline

Pipeline section allows user to customize the Tekton pipeline features. This allow user to customize the values in configmaps.

Refer to [properties](./TektonPipeline.md#properties) section in TektonPipeline for available options.

Some of the fields have default values, so operator will add them if user haven't passed in CR. Other fields which
doesn't have default values won't be passed unless user specify them in CR. User can find those [here](./TektonPipeline.md#optional-properties).

Example:

```yaml
pipeline:
  disable-creds-init: false
  disable-home-env-overwrite: true
  disable-working-directory-overwrite: true
  enable-api-fields: stable
  enable-custom-tasks: false
  metrics.pipelinerun.duration-type: histogram
  metrics.pipelinerun.level: pipelinerun
  metrics.taskrun.duration-type: histogram
  metrics.taskrun.level: taskrun
  traces.enabled: true
  traces.endpoint: "http://jaeger-collector.jaeger.svc.cluster.local:4318/v1/traces"
  traces.credentialsSecret: "" # optional
  require-git-ssh-secret-known-hosts: false
  running-in-environment-with-injected-sidecars: true
  trusted-resources-verification-no-match-policy: ignore
  performance:
    replicas: 1
    disable-ha: false
    buckets: 1
    threads-per-controller: 2
    kube-api-qps: 5.0
    kube-api-burst: 10
```

### Chain

Chain section allows user to customize the Tekton Chain features. This allows user to customize the values in configmaps

Refer to [properties](https://github.com/tektoncd/operator/blob/main/docs/TektonChain.md#chain-config) section in TektonChain for available options

Example:

```yaml
chain:
  disabled: false # - `disabled` : if the value set as `true`, chains feature will be disabled (default: `false`)
  targetNamespace: tekton-pipelines
  generateSigningSecret: true # default value: false
  controllerEnvs:
    - name: MONGO_SERVER_URL # This is the only field supported at the moment which is optional and when added by user, it is added as env to Chains controller
      value: #value               # This can be provided same as env field of container
  artifacts.taskrun.format: in-toto
  artifacts.taskrun.storage: tekton,oci (comma separated values)
  artifacts.taskrun.signer: x509
  artifacts.oci.storage: oci (comma separated values)
  artifacts.oci.format: simplesigning
  artifacts.oci.signer: x509
  artifacts.pipelinerun.format: in-toto
  artifacts.pipelinerun.storage: tekton,oci (comma separated values)
  artifacts.pipelinerun.signer: x509
  storage.gcs.bucket: #value
  storage.oci.repository: #value
  storage.oci.repository.insecure: #value (boolean - true/false)
  storage.docdb.url: #value
  storage.grafeas.projectid: #value
  storage.grafeas.noteid: #value
  storage.grafeas.notehint: #value
  builder.id: #value
  signers.x509.fulcio.enabled: #value (boolean - true/false)
  signers.x509.fulcio.address: #value
  signers.x509.fulcio.issuer: #value
  signers.x509.fulcio.provider: #value
  signers.x509.identity.token.file: #value
  signers.x509.tuf.mirror.url: #value
  signers.kms.kmsref: #value
  signers.kms.kmsref.auth.address: #value
  signers.kms.kmsref.auth.token: #value
  signers.kms.kmsref.auth.oidc.path: #value
  signers.kms.kmsref.auth.oidc.role: #value
  signers.kms.kmsref.auth.spire.sock: #value
  signers.kms.kmsref.auth.spire.audience: #value
  transparency.enabled: #value (boolean - true/false)
  transparency.url: #value
```

### Result

Result section allows user to customize the Tekton Result component, Refer to [Result Spec](https://github.com/tektoncd/operator/blob/main/docs/TektonResult.md#spec) section in TektonResult for available options.

Default Result configuration in TektonConfig looks like following if user doesn't specified any configuration options

Example:

```yaml
result:
  disabled: false
  is_external_db: false
  options: {}
```

User can customize Result configuration with following options

Example:

```yaml
result:
  disabled: false # - `disabled` : if the value set as `true`, result component will be disabled (default: `false`)
  targetNamespace: tekton-pipelines
  is_external_db: false # By default, this is set to false, TektonOperator will create Tekton Results database. If set to true, an external database will be used, and Tekton Results will retrieve its database credentials from the Kubernetes secret named tekton-results-postgres
  db_host: localhost
  db_port: 5342
  db_sslmode: verify-full
  db_sslrootcert: /etc/tls/db/ca.crt
  db_enable_auto_migration: true
  log_level: debug
  logs_api: true
  logs_type: File
  logs_buffer_size: 90kb
  logs_path: /logs
  auth_disable: true
  logging_pvc_name: tekton-logs
  secret_name: # optional
  gcs_creds_secret_name: <value>
  gcc_creds_secret_key: <value>
  gcs_bucket_name: <value>
  loki_stack_name: #optional
  loki_stack_namespace: #optional
  prometheus_port: 9090
  prometheus_histogram: false
```

User can configure custom database secret name for internal/external database via Tekton Config CR.

Example:

```yaml
result:
  db_secret_name: # custom database secret name
  db_secret_user_key: # optional: required if custom database secret user key is not "POSTGRES_USER"
  db_secret_password_key: # optional: required if custom database secret password key is not "POSTGRES_PASSWORD"
```

### Pruner

Pruner provides auto clean up feature for the Tekton `pipelinerun` and `taskrun` resources. In the background pruner container runs `tkn` command.

Example:

```yaml
pruner:
  disabled: false
  schedule: "0 8 * * *"
  startingDeadlineSeconds: 100 # optional
  resources:
    - taskrun
    - pipelinerun
  keep: 3
  # keep-since: 1440
  # NOTE: you can use either "keep" or "keep-since", not both
  prune-per-resource: true
```

- `disabled` : if the value set as `true`, pruner feature will be disabled (default: `false`)
- `schedule`: how often to run the pruner job. User can understand the schedule syntax [here][schedule].
- `startingDeadlineSeconds`: Optional deadline in seconds for starting the job if it misses scheduled time for any reason. Missed jobs executions will be counted as failed ones
- `resources`: supported resources for auto prune are `taskrun` and `pipelinerun`
- `keep`: maximum number of resources to keep while deleting or removing resources
- `keep-since`: retain the resources younger than the specified value in minutes
- `prune-per-resource`: if the value set as `true` (default value `false`), the `keep` applied to each resource. The resources(`pipeline` and/or `task`) taken dynamically from that namespace and applied. <br> example: in a namespace `ns-1` I have two `pipeline`, named `pipeline-1` and `pipeline-2`, the out come would be: `tkn pipelinerun delete --pipeline=my-pipeline-1 --keep=3 --namespace=ns-1`, `tkn pipelinerun delete --pipeline=my-pipeline-2 --keep=3 --namespace=ns-1`. the same way works for `task` too.<br> **We do not see any benefit by enabling `prune-per-resource=true`, when you use `keep-since`. As `keep-since` is limiting the resources by time(irrespective of resource count), there is no change on the outcome.**

> ### Note:
>
> if `disabled: false` and `schedule: ` with empty value, global pruner job will be disabled.
> however, if there is a prune schedule (`operator.tekton.dev/prune.schedule`) annotation present with a value in a namespace. a namespace wide pruner jobs will be created.

#### Pruner Namespace annotations

By default pruner job will be created from the global pruner config (`spec.pruner`), though user can customize a pruner config to a specific namespace with the following annotations. If some of the annotations are not present or has invalid value, for that value, falls back to global value or skipped the namespace.

- `operator.tekton.dev/prune.skip` - pruner job will be skipped to a namespace, if the value set as `true`
- `operator.tekton.dev/prune.schedule` - pruner job will be created on a specific schedule
- `operator.tekton.dev/prune.keep` - maximum number of resources will be kept
- `operator.tekton.dev/prune.keep-since` - retain the resources younger than the specified value in minutes
- `operator.tekton.dev/prune.prune-per-resource` - the `keep` or `keep-since` applied to each resource
- `operator.tekton.dev/prune.resources` - can be `taskrun` and/or `pipelinerun`, both value can be specified with comma separated. example: `taskrun,pipelinerun`
- `operator.tekton.dev/prune.strategy` - allowed values: either `keep` or `keep-since`

> ### Note:
>
> if a global value is not present the following values will be consider as default value <br> > `resources: pipelinerun` <br> > `keep: 100` <br>

### Scheduler

Scheduler section allows you to install and manage the [Tekton Scheduler](./TektonScheduler.md) through TektonConfig. The Scheduler component uses [Kueue](https://kueue.sigs.k8s.io) and [cert-manager](https://github.com/cert-manager/cert-manager); you must install Kueue and cert-manager CRDs before enabling the scheduler. For full pre-requisites and multi-cluster configuration details, see [Tekton Scheduler](./TektonScheduler.md).

Scheduler can be enabled by setting `disabled` to `false` in the scheduler section. If you are working with multi-cluster pipelines, you can enable multi-cluster from the scheduler config. In a multi-cluster environment a cluster can play the role of **Hub** or **Spoke**. The TektonConfig settings for Scheduler for Hub and Spoke are defined below.

#### Hub cluster

```yaml
scheduler:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Hub
  options: {}
```

#### Spoke cluster

```yaml
scheduler:
  disabled: false
  multi-cluster-disabled: false
  multi-cluster-role: Spoke
  options: {}
```

- `disabled`: set to `false` to enable the Scheduler component (default is `true`).
- `multi-cluster-disabled`: when `false`, multi-cluster features are enabled (default is `true`).
- `multi-cluster-role`: `Hub` or `Spoke`. When set to **Hub**, TektonConfig also creates and manages the [TektonMulticlusterProxyAAE](./TektonMulticlusterProxyAAE.md) component automatically (the proxy is used to communicate with spoke clusters, e.g. for [Kueue MultiKueue](https://kueue.sigs.k8s.io/docs/concepts/multikueue/)). On spoke clusters use `Spoke`; the proxy is not installed there.

See [TektonMulticlusterProxyAAE](./TektonMulticlusterProxyAAE.md) for details on the proxy component and its requirements.

### Addon

TektonAddon install some resources along with Tekton Pipelines on the cluster. This provides few PipelineTemplates, ResolverTasks, ResolverStepActions and CommunityResolverTasks.

This section allows to customize installation of those resources through params. You can read more about the supported params [here](./TektonAddon.md).

Example:

```yaml
addon:
  params:
    - name: "pipelineTemplates"
      value: "true"
    - name: "resolverTasks"
      value: "true"
    - name: "resolverStepActions"
      value: "true"
    - name: "communityResolverTasks"
      value: "true"
```

**NOTE**: TektonAddon is currently available for OpenShift Platform only. Enabling this for Kubernetes platform is in roadmap
of Operator.

### Hub

This is to enable/disable showing hub resources in pipeline builder of devconsole(OpenShift UI). By default, the field is
not there in the config object. If you want to disable the integration, you can add the param like below in config with value `false`.
The possible values are `true` and `false`.

Example:

```yaml
hub:
  params:
    - name: enable-devconsole-integration
      value: "false"
```

### Dashboard

Dashboard provides configuration options for the Tekton Dashboard if the specified profile value includes the Dashboard component. (E.g. `all` on Kubernetes)

Example:

```yaml
dashboard:
  readonly: true
```

- `readonly`: If set to true, install the Dashboard in read-only mode

This is an `Optional` section.

### Resolvers

As part of TektonPipelines, resolvers are installed which are by default enabled. User can disable them through TektonConfig.

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  pipeline:
    enable-bundles-resolver: true
    enable-cluster-resolver: true
    enable-git-resolver: true
    enable-hub-resolver: true
```

User can also provide resolver specific configurations through TektonConfig. The Default configurations are **not** added by default in TektonConfig.
To override default configurations, user can provide the configurations as below. You can find the default configurations [here](https://github.com/tektoncd/pipeline/tree/main/config)

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  pipeline:
    bundles-resolver-config:
      default-service-account: pipelines
    cluster-resolver-config:
      default-namespace: cluster-resolver-test
    enable-bundles-resolver: true
    enable-cluster-resolver: true
    enable-git-resolver: true
    enable-hub-resolver: true
    git-resolver-config:
      server-url: localhost.com
    hub-resolver-config:
      default-tekton-hub-catalog: tekton
      tekton-hub-api: "https://my-custom-tekton-hub.example.com"
      artifact-hub-api: "https://my-custom-artifact-hub.example.com"
```

Under the `hub-resolver-config`, the `tekton-hub-api` and `artifact-hub-api` will be passed as environment variable into `tekton-pipelines-remote-resolvers` controller.<br>
In the deployment the environment name will be converted as follows,

- `tekton-hub-api` => `TEKTON_HUB_API`
- `artifact-hub-api` => `ARTIFACT_HUB_API`

### OpenShiftPipelinesAsCode

The PipelinesAsCode section allows you to customize the Pipelines as Code features. When you change the TektonConfig CR, the Operator automatically applies the settings to custom resources and configmaps in your installation.

Some of the fields have default values, so operator will add them if the user hasn't passed in CR. Other fields which
don't have default values unless the user specifies them. User can find those [here](https://pipelinesascode.com/docs/install/settings/#pipelines-as-code-configuration-settings).

Example:

```yaml
platforms:
  openshift:
    pipelinesAsCode:
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
        error-detection-simple-regexp:
          ^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([
          ]*)?(?P<error>.*)
        error-log-snippet: "true"
        enable-cancel-in-progress-on-pull-requests: "false"
        enable-cancel-in-progress-on-push: "false"
        hub-catalog-name: tekton
        hub-url: https://api.hub.tekton.dev/v1
        require-ok-to-test-sha: "false"
        remote-tasks: "true"
        secret-auto-create: "true"
        secret-github-app-token-scoped: "true"
```

#### Remote Hub Catalogs

Pipelines as Code supports configuring remote hub catalogs to fetch tasks and pipelines. You can configure custom catalogs using the `catalog-{INDEX}-*` settings pattern.

Each custom catalog requires the following four fields:

| Field | Description | Required |
|-------|-------------|----------|
| `catalog-{INDEX}-id` | Unique identifier for the catalog. Users reference tasks from this catalog using this ID as prefix (e.g., `custom://task-name`) | Yes |
| `catalog-{INDEX}-name` | Name of the catalog within the hub | Yes |
| `catalog-{INDEX}-url` | URL endpoint of the hub API | Yes |
| `catalog-{INDEX}-type` | Type of hub: `artifacthub` (for Artifact Hub) or `tektonhub` (for Tekton Hub) | Yes |

Where `{INDEX}` is a number that can be incremented to add multiple catalogs (e.g., `1`, `2`, `3`).

##### Example: Configuring a Custom Hub Catalog

```yaml
platforms:
  openshift:
    pipelinesAsCode:
      enable: true
      settings:
        catalog-1-id: "custom"
        catalog-1-name: "tekton"
        catalog-1-url: "https://api.custom.hub/v1"
        catalog-1-type: "tektonhub"
```

##### Example: Configuring Multiple Hub Catalogs

You can configure multiple hub catalogs by incrementing the `catalog-{INDEX}` number:

```yaml
platforms:
  openshift:
    pipelinesAsCode:
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
```

##### Referencing Tasks from Custom Catalogs

Once configured, users can reference tasks from custom catalogs by adding a prefix matching the catalog ID:

```yaml
pipelinesascode.tekton.dev/task: "custom://git-clone"
pipelinesascode.tekton.dev/task: "artifact://buildah"
```

> **Note:** Pipelines-as-Code will not try to fallback to the default or another custom hub if the task referenced is not found (the Pull Request will be set as failed).

For more details, see the [Pipelines-as-Code Remote Hub Catalogs documentation](https://pipelinesascode.com/docs/install/settings/#remote-hub-catalogs).

**NOTE**: OpenShiftPipelinesAsCode is currently available for the OpenShift Platform only.

### Event based pruner 

The `tektonpruner` section in the TektonConfig spec allows you to manage the event-driven Tekton Pruner, which enables configuration-based cleanup of Tekton resources such as PipelineRuns and TaskRuns.

> Important: This component is **disabled by default**. To enable the event-based pruner, the existing job-based pruner `pruner` **MUST** be disabled.

#### Basic Configuration

```yaml
  pruner:
    disabled: true  # Must disable job-based pruner
  tektonpruner:
    disabled: false  # Enable event-based pruner
    global-config:
      enforcedConfigLevel: global  # Options: global, namespace
      ttlSecondsAfterFinished: 3600  # Delete runs older than 1 hour (optional)
      historyLimit: 100              # Keep only 100 runs total (optional)
      successfulHistoryLimit: 50     # Keep only 50 successful runs (optional)
      failedHistoryLimit: 20         # Keep only 20 failed runs (optional)
    options: {}
```

#### Configuration Levels

The `enforcedConfigLevel` determines the configuration hierarchy:

- **`global`**: Cluster-wide defaults apply to all namespaces (no namespace overrides allowed)
- **`namespace`**: Allows namespace-level overrides via ConfigMaps in individual namespaces

#### Pruning Fields

You can specify any combination of these fields. The pruner deletes runs when **any one** of the specified conditions is met:

- **`ttlSecondsAfterFinished`**: Time in seconds to retain completed runs before pruning
- **`historyLimit`**: Maximum number of runs to retain for each status (applies independently to both successful and failed runs)
- **`successfulHistoryLimit`**: Maximum number of successful runs to retain
- **`failedHistoryLimit`**: Maximum number of failed runs to retain

**Note:** If `successfulHistoryLimit` or `failedHistoryLimit` is not specified, `historyLimit` value is used as fallback for that status type.

#### Namespace-Level Configuration

Configure different default pruning policies for specific namespaces in the global config:

```yaml
  tektonpruner:
    disabled: false
    global-config:
      enforcedConfigLevel: namespace
      historyLimit: 100  # Global default
      namespaces:
        dev-namespace:
          historyLimit: 50
          ttlSecondsAfterFinished: 1800  # 30 minutes for dev
        prod-namespace:
          successfulHistoryLimit: 200
          failedHistoryLimit: 50
          ttlSecondsAfterFinished: 86400  # 24 hours for prod
```

> **Note:** For advanced configurations including resource-level selectors and per-namespace overrides via ConfigMaps, refer to the [TektonPruner documentation](./TektonPruner.md).

#### Complete Example

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  targetNamespace: tekton-pipelines
  profile: all
  pruner:
    disabled: true  # Disable job-based pruner
  tektonpruner:
    disabled: false
    global-config:
      enforcedConfigLevel: namespace
      historyLimit: 100
      ttlSecondsAfterFinished: 7200  # 2 hours
      namespaces:
        tekton-pipelines:
          historyLimit: 200
          successfulHistoryLimit: 150
          failedHistoryLimit: 50
        dev:
          ttlSecondsAfterFinished: 3600  # 1 hour
          historyLimit: 50
    options:
      disabled: false
      deployments:
        tekton-pruner-controller:
          spec:
            replicas: 1
```

**Configuration Notes:**
- Both pruners (job-based and event-based) cannot be enabled simultaneously
- The event-based pruner responds to resource events in real-time, providing more efficient cleanup
- When `enforcedConfigLevel` is set to `namespace`, individual namespaces can override these settings using ConfigMaps



### Additional fields as `options`

There is a field called `options` available in all the components.<br>

> **NOTE:** There is a possibility to have two different values for a field.<br>
> An example: with a pre-defined field you can set value and the same field may be defined under `options` as well. In that case value from `options` will be final.

A sample `options` field,

```yaml
options:
  disabled: false
  configMaps:
    config-leader-election: # name of the configMap
      data:
        lease-duration: "90s"
    pipeline-config-logging: # creates new configMap under targetNamespace
      metadata:
        labels:
          my-custom-logger: "true"
        annotations:
          logger-type: "uber-zap"
      data:
        loglevel.controller: "info"
        loglevel.webhook: "info"
        zap-logger-config: |
          {
            "level": "debug",
            "development": false,
            "sampling": {
              "initial": 100,
              "thereafter": 50
            },
            "outputPaths": ["stdout"],
            "errorOutputPaths": ["stderr"],
            "encoding": "json",
            "encoderConfig": {
              "timeKey": "ts",
              "levelKey": "severity",
              "nameKey": "logger",
              "callerKey": "caller",
              "messageKey": "message",
              "stacktraceKey": "stacktrace",
              "lineEnding": "",
              "levelEncoder": "",
              "timeEncoder": "iso8601",
              "durationEncoder": "",
              "callerEncoder": ""
            }
          }

  deployments:
    tekton-pipelines-controller: # name of the deployment
      metadata:
        labels:
          custom-label: "foo"
        annotations:
          custom-annotation: "foo"
      spec:
        replicas: 2
        template:
          spec:
            containers:
              - name: tekton-pipelines-controller
                env:
                  - name: CONFIG_LOGGING_NAME
                    value: pipeline-config-logging
  statefulSets:
    web: # name of the statefulSets
      metadata:
        labels:
          custom-label: foo
        annotations:
          custom-annotation: foo
      spec:
        replicas: 3
        template:
          spec:
            containers:
              - name: nginx
                env:
                  - name: NGINX_MODE
                    value: production
  horizontalPodAutoscalers:
    tekton-pipelines-webhook: # name of the hpa
      metadata:
        annotations:
        labels:
      spec:
        minReplicas: 2
        maxReplicas: 7
        metrics:
          - resource:
              name: cpu
              target:
                averageUtilization: 85
                type: Utilization
            type: Resource
  webhookConfigurationOptions:
    validation.webhook.pipeline.tekton.dev:
      failurePolicy: Fail
      timeoutSeconds: 20
      sideEffects: None
    webhook.pipeline.tekton.dev:
      failurePolicy: Fail
      timeoutSeconds: 20
      sideEffects: None
```

- `disabled` - disables the additional `options` support, if `disabled` set to `true`. default: `false`

#### ConfigMaps

Supports to update existing configMap also supports to create new configMap.

The following fields are supported in `configMap`

- `metadata`
  - `labels` - supports add and update
  - `annotations` - supports add and update
- `data` - supports add and update

#### Deployments

Supports to update the existing deployments. But not supported to create new deployment.

The following fields are supported in `deployment`

- `metadata`
  - `labels` - supports add and update
  - `annotations` - supports add and update
- `spec`
  - `replicas` - updates deployment replicas count
  - `template`
    - `metadata`
      - `labels` - supports add and update
      - `annotations` - supports add and update
    - `spec`
      - `affinity` - replaces the existing Affinity with this, if not empty
      - `priorityClassName` - replaces the existing PriorityClassName with this, if not empty
      - `nodeSelector` - replaces the existing NodeSelector with this, if not empty
      - `tolerations` - replaces the existing tolerations with this, if not empty
      - `topologySpreadConstraints` - replaces the existing TopologySpreadConstraints with this, if not empty
      - `runtimeClassName` - adds and updates runtimeClassName
      - `volumes` - adds and updates volumes
      - `initContainers` - adds and updates init-containers
        - `resources` - replaces the resources requirements with this, if not empty
        - `envs` - adds and updates environments
        - `volumeMounts` - adds and updates VolumeMounts
        - `args` - appends given args with existing arguments. **NOTE: THIS OPERATION DO NOT REPLACE EXISTING ARGS**
      - `containers` - adds and updates containers
        - `resources` - replaces the resources requirements with this, if not empty
        - `envs` - adds and updates environments
        - `volumeMounts` - adds and updates VolumeMounts
        - `args` - appends given args with existing arguments. **NOTE: THIS OPERATION DO NOT REPLACE EXISTING ARGS**

#### StatefulSets

Supports to update the existing StatefulSet. But not supported to create new StatefulSet.

The following fields are supported in `StatefulSet`

- `metadata`
  - `labels` - supports add and update
  - `annotations` - supports add and update
- `spec`
  - `replicas` - updates statefulSets replicas count
  - `serviceName` - updates service name
  - `podManagementPolicy` - updates pod management policy
  - `volumeClaimTemplates` - updates volume claim templates
  - `template`
    - `metadata`
      - `labels` - supports add and update
      - `annotations` - supports add and update
    - `spec`
      - `affinity` - replaces the existing Affinity with this, if not empty
      - `priorityClassName` - replaces the existing PriorityClassName with this, if not empty
      - `nodeSelector` - replaces the existing NodeSelector with this, if not empty
      - `tolerations` - replaces the existing tolerations with this, if not empty
      - `topologySpreadConstraints` - replaces the existing TopologySpreadConstraints with this, if not empty
      - `runtimeClassName` - adds and updates runtimeClassName
      - `volumes` - adds and updates volumes
      - `initContainers` - adds and updates init-containers
        - `resources` - replaces the resources requirements with this, if not empty
        - `envs` - adds and updates environments
        - `volumeMounts` - adds and updates VolumeMounts
        - `args` - appends given args with existing arguments. **NOTE: THIS OPERATION DO NOT REPLACE EXISTING ARGS**
      - `containers` - adds and updates containers
        - `resources` - replaces the resources requirements with this, if not empty
        - `envs` - adds and updates environments
        - `volumeMounts` - adds and updates VolumeMounts
        - `args` - appends given args with existing arguments. **NOTE: THIS OPERATION DO NOT REPLACE EXISTING ARGS**

#### HorizontalPodAutoscalers

Supports to update the existing HorizontalPodAutoscaler(HPA) also supports to create new HPA.

The following fields are supported in `HorizontalPodAutoscaler` (aka HPA)

- `metadata`
  - `labels` - supports add and update
  - `annotations` - supports add and update
- `spec`
  - `scaleTargetRef` - replaces scaleTargetRef with this, if `kind` and `name` are not empty
  - `minReplicas` - updates minimum replicas count
  - `maxReplicas` - updates maximum replicas count
  - `metrics` - replaces the metrics details with this array, if not empty
  - `behavior` - updates behavior data with this, if not empty
    - `scaleUp` - replaces scaleUp with this, if not empty
    - `scaleDown` - replaces scaleDown with this, if not empty

**NOTE**: If a Deployment or StatefulSet has a Horizontal Pod Autoscaling (HPA) and is in active state, Operator will not control the replicas to that resource. However if `status.desiredReplicas` and `spec.minReplicas` not present in HPA, operator takes the control. Also if HPA disabled, operator takes control. Even though the operator takes the control, the replicas value will be adjusted to the hpa's scaling range.

#### webhookConfigurationOptions

Defines additional options for each webhooks. Use webhook name as a key to define options for a webhook. Options are ignored if the webhook does not exist with the name key. To get detailed information about webhooks options visit https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/

the following options are supported for webhookConfigurationOptions

- `failurePolicy` - defines how unrecognized errors and timeout errors from the admission webhook are handled. Allowed values are `Ignore` or `Fail`
- `timeoutSeconds` - allows configuring how long the API server should wait for a webhook to respond before treating the call as a failure.
- `sideEffects` - indicates whether the webhook have a side effet. Allowed values are `None`, `NoneOnDryRun`, `Unknown`, or `Some`

[node-selector]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
[tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
[schedule]: https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#cron-schedule-syntax
[priorityClassName]: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#pod-priority
[priorityClass]: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass
