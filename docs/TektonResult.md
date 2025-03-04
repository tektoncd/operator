<!--
---
linkTitle: "TektonResult"
weight: 5
---
-->
# Tekton Result

TektonResult custom resource allows user to install and manage [Tekton Result][result].

TektonResult is installed through [TektonConfig](./TektonConfig.md) by default.

**Note** : TektonOperator creates a secret for default database root password and a tls secret for TektonResult, the TektonResult doesn't rotate the tls certificate.

- Create PVC if using PVC for logging (Optional)

```!bash
cat <<EOF > pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tekton-logs
  namespace: tekton-pipelines
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF
// Apply the above PVC
kubectl apply -f pvc.yaml
```

- Check the status of installation using following command
  ```sh
  kubectl get tektonresults.operator.tekton.dev
  ```

## Spec
The TektonResult CR is like below:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonResult
metadata:
  name: result
spec:
  targetNamespace: tekton-pipelines
  db_host: localhost
  db_port: 5342
  db_sslmode: verify-full
  db_sslrootcert: /etc/tls/db/ca.crt
  db_enable_auto_migration: true
  db_secret_name: # optional: custom database secret name
  db_secret_user_key: # optional
  db_secret_password_key: # optional
  log_level: debug
  logs_api: true
  logs_type: File
  logs_buffer_size: 32768
  logs_path: /logs
  auth_disable: true
  logging_pvc_name: tekton-logs
  secret_name: # optional
  gcs_creds_secret_name: <value>
  gcc_creds_secret_key: <value>
  gcs_bucket_name: <value>
  is_external_db: false
  loki_stack_name: #optional
  loki_stack_namespace: #optional
  prometheus_port: 9090
  prometheus_histogram: false
```

These properties are analogous to the one in configmap of tekton results api `tekton-results-api-config` documented at [api.md](https://github.com/tektoncd/results/blob/4472848a0fb7c1473cfca8b647553170efac78a1/cmd/api/README.md)


[result]:https://github.com/tektoncd/results


### Property "secret_name":
`secret_name` - name of your custom secret or leave it as empty. It an optional property. The secret should be created by the user on the `targetNamespace`. The secret can contain `S3_` prefixed keys from the [result API properties](https://github.com/tektoncd/results/blob/fded140081468e418aeb860d16aca3306c675d8b/cmd/api/README.md). Please note: the key of the secret should be in UPPER_CASE and values should be in `string` format.
The following keys are supported by this secret.
* `S3_BUCKET_NAME`
* `S3_ENDPOINT`
* `S3_HOSTNAME_IMMUTABLE`
* `S3_REGION`
* `S3_ACCESS_KEY_ID`
* `S3_SECRET_ACCESS_KEY`
* `S3_MULTI_PART_SIZE`

#### Sample Secret File
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my_custom_secret
  namespace: tekton-pipelines
type: Opaque
stringData:
  S3_BUCKET_NAME: foo
  S3_ENDPOINT: https://example.localhost.com
  S3_HOSTNAME_IMMUTABLE: "false"
  S3_REGION: region-1
  S3_ACCESS_KEY_ID: "1234"
  S3_SECRET_ACCESS_KEY: secret_key
  S3_MULTI_PART_SIZE: "5242880"
```


### GCS specific Property
The follow keys are needed for enabling GCS storage of logs:
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonResult
metadata:
  name: result
spec:
  gcs_creds_secret_name: <value>
  gcc_creds_secret_key: <value>
  gcs_bucket_name: <value>
```

We need to create a secret with google application creds for a bucket `foo-bar` like below:

```
kubectl create secret generic gcs-credentials --from-file=creds.json
```

To know more about Application Default Credentials in `creds.json` that is use to create above secret for GCS, please visit: https://cloud.google.com/docs/authentication/application-default-credentials

In the above example, our properties are:

```
gcs_creds_secret_name: gcs-credentials
gcc_creds_secret_key: creds.json
gcs_bucket_name: foo-bar
```

### External DB

It is not recommended to use internal DB, operator hard code PVC configuration and DB settings.

If you want to move from internal DB to external DB, please take backup of the DB. If you want to start fresh, then
delete previous TektonResult CR. and recreate the fresh one with following instructions:

- Generate a secret with user name and password for Postgres (subsitute ${password} with your password):
```sh
   export NAMESPACE="tekton-pipelines" # Put the targetNamespace of TektonResult where it is going to be installed.
   kubectl create secret generic tekton-results-postgres --namespace=${NAMESPACE} --from-literal=POSTGRES_USER=result --from-literal=POSTGRES_PASSWORD=${password}
```

- Create a TektonResult CR like below:
* Add `db_host` with DB url without port.
* Add `db_port` with your DB port.
* Set `is_external_db` to true.
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonResult
metadata:
  name: result
spec:
  targetNamespace: tekton-pipelines
  db_port: 5432
  db_user: result
  db_host: tekton-results-postgres-external-service.pg-redhat.svc.cluster.local
  is_external_db: true
...
```

### Securing the DB connection

To secure the DB connection using self-segned certificate or using certificate signed by 3rd party CA (e.g AWS RDS), one can provide path to the DB SSL root certificate, mounted and available on the Results API pod. The configuration will look like:


```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonResult
metadata:
  name: result
spec:
  targetNamespace: tekton-pipelines
  db_host: tekton-results-postgres-service.openshift-pipelines.svc.cluster.local
  db_port: 5342
  db_sslmode: verify-full
  db_sslrootcert: /etc/tls/db/ca.crt
  ...
```

The valid options for `db_sslmode` are explained here https://www.postgresql.org/docs/current/libpq-ssl.html#LIBPQ-SSL-PROTECTION. To use any of the `require`, `verify-ca` and `verify-full` modes with self signed certificate, the path to the CA certificate which signed the DB certificate must be provided as `db_sslrootcert`.

## LokiStack + TektonResult

Tekton Results leverages external Third Party APIs to query data. Storing of data via Tekton Results is inefficient
and bad for performance. It's better to use forwarders like Vector, Promtail, Fluentd for forwarding TaskRun pod Logs from nodes.

### Kubernetes (GCP) + LokiStack/Loki

#### Loki

You can either use Grafana's [Helm Repo](https://grafana.com/docs/loki/latest/setup/install/helm/) or operator from [OperatorHub](https://operatorhub.io/operator/loki-operator) to install Loki. 
Installing via operator simplies certain operations for Tekton Operator. You just need to configure `lokistack_name` and `lokistack_namespace`.

In case of helm installation, you will need to configure options field to configure Results API configMap `tekton-results-api-config`:
```yaml
LOGS_API
LOGGING_PLUGIN_PROXY_PATH
LOGGING_PLUGIN_API_URL
LOGGING_PLUGIN_TOKEN_PATH
LOGGING_PLUGIN_NAMESPACE_KEY
LOGGING_PLUGIN_STATIC_LABELS
LOGGING_PLUGIN_TLS_VERIFICATION_DISABLE
LOGGING_PLUGIN_FORWARDER_DELAY_DURATION
LOGGING_PLUGIN_QUERY_PARAMS
LOGGING_PLUGIN_QUERY_LIMIT
```

Please consult the docs [here](https://github.com/tektoncd/results/blob/main/docs/logging-support.md) for information on these fields.

These fields allow you to configure how Tekton Results interacts with your Loki backend.

You might need to configure following environment variable to Tekton Results API deployment if you are using some custom CA to generate TLS certificate:
```yaml
LOGGING_PLUGIN_CA_CERT
```

- `LOGGING_PLUGIN_FORWARDER_DELAY_DURATION`: This is the max duration in minutes taken by third party logging system to forward and store the logs after completion of taskrun and pipelinerun. This is used to search between start time of runs and completion plus buffer duration.

#### Forwarder

You need to configure forwarder systems to add labels for namespace, pass TaskRun UID/PipelineRun UID in pods and a common label <key:value> alongwith logs from nodes.

A sample configuration for vector: [values.yaml](https://github.com/tektoncd/results/blob/main/test/e2e/loki_vector/vector.yaml). 

### OpenShift (LokiStack + OpenShift Logging)


To configure LokiStack with TektonResult, you can use the `lokistack_name` and `lokistack_namespace` properties in the TektonResult custom resource. Here's how to do it:


1. First, ensure that you have LokiStack installed in your cluster.

2. Then, create or update your TektonResult CR with the following properties:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonResult
metadata:
  name: result
spec:
  targetNamespace: tekton-pipelines
  // ... other properties ...
  lokistack_name: your-lokistack-name
  lokistack_namespace: your-lokistack-namespace
```
Replace your-lokistack-name with the name of your LokiStack instance and your-lokistack-namespace with the namespace where LokiStack is installed.

By setting these properties, Operator will configure Tekton Result to use the specified LokiStack instance for log retrieval.


#### OpenShift Logging

Install the openshift logging operator by following this: [Deploying Cluster Logging](https://docs.openshift.com/container-platform/4.16/observability/logging/cluster-logging-deploying.html#logging-loki-gui-install_cluster-logging-deploying)

If you are installing OpenShift Logging Operator only for TaskRun Logs, then you also need to configure a ClusterLogForwarder:
```yaml
apiVersion: "logging.openshift.io/v1"
kind: ClusterLogForwarder
metadata:
  name: instance
  namespace: openshift-logging
spec:
  inputs:
  - name: only-tekton
    application:
      selector:
        matchLabels:
          app.kubernetes.io/managed-by: tekton-pipelines
  pipelines:
    - name: enable-default-log-store
      inputRefs: [ only-tekton ]
      outputRefs: [ default ]
```
### Tekton Result Watcher Performance Properties

Tekton Results Watcher can now be deployed using StatefulSet ordinals to partition work among replicas. When enabled, this feature uses stable pod identities (ordinals) to determine which pod is responsible for reconciling a given set of resources, instead of relying solely on traditional leader election. This can improve work distribution under load and provides a deterministic mechanism for controller coordination.

#### Results Watcher Performance Configuration
```yaml
spec:
  # omitted other fields ...
  performance:
    disable-ha: false
    buckets: 1
    replicas: 1
    statefulset-ordinals: false

```
These fields are optional and there is no default values. If user passes them, operator will include most of fields into the deployment `tekton-results-watcher` under the container `watcher` as arguments(duplicate name? No, container and deployment has the same name), otherwise result watcher controller's default values will be considered. and `buckets` field is updated into `tekton-results-config-leader-election` config-map under the namespace `tekton-pipelines`.

* `disable-ha` - enable or disable ha feature, defaults in results watcher controller is `disable-ha=false`
* `buckets` - buckets is the number of buckets used to partition key space of each reconciler. If this number is M and the replica number of the controller is N, the N replicas will compete for the M buckets. The owner of a bucket will take care of the reconciling for the keys partitioned into that bucket. The maximum value of `buckets` at this time is `10`. default value in pipeline controller is `1`
* `replicas` - results watcher controller deployment replicas count
* `statefulset-ordinals` - enables StatefulSet Ordinals mode for the Tekton Results Watcher Controller. When set to true, the Results Watcher Controller is deployed as a StatefulSet, allowing for multiple replicas to be configured with a load-balancing mode. This ensures that the load is evenly distributed across replicas, and the number of buckets is enforced to match the number of replicas.
  Moreover, There are two mechanisms available for scaling Results Watcher Controller horizontally:
- Using leader election, which allows for failover, but can result in hot-spotting.
- Using StatefulSet ordinals, which doesn't allow for failover, but guarantees load is evenly spread across replicas.


> #### Note:
> * if you modify or remove any of the performance properties, `tekton-results-watcher` deployment and `tekton-results-config-leader-election` config-map (if `buckets` changed) will be updated, and `tekton-results-watcher` pods will be recreated

### Debugging

#### Debugging gRPC

Set `prometheus_histogram: true` to turns on recording of handling time of RPCs. Histogram metrics can be very expensive for Prometheus to retain and query. Disabled by default.
