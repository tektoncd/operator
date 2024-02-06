<!--
---
linkTitle: "TektonResult"
weight: 5
---
-->
# Tekton Result

TektonResult custom resource allows user to install and manage [Tekton Result][result].

TektonResult is an optional component and currently cannot be installed through TektonConfig. It has to be installed seperately.

To install Tekton Result on your cluster follow steps as given below:
- Make sure Tekton Pipelines is installed on your cluster, using the Operator.
- Generate a database root password.
  A database root password must be generated and stored in a [Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/)
  before installing results. By default, Tekton Results expects this secret to have
  the following properties:

    - namespace: `tekton-pipelines`
    - name: `tekton-results-postgres`
    - contains the fields:
        - `user=<user name>`
        - `password=<your password>`

  If you are not using a particular password management strategy, the following
  command will generate a random password for you:
  Update namespace value in the command if Tekton Pipelines is installed in a different namespace..

   ```sh
   export NAMESPACE="tekton-pipelines"
   kubectl create secret generic tekton-results-postgres --namespace=${NAMESPACE} --from-literal=POSTGRES_USER=result --from-literal=POSTGRES_PASSWORD=$(openssl rand -base64 20)
   ```
- Generate cert/key pair.
  Note: Feel free to use any cert management software to do this!

  Tekton Results expects the cert/key pair to be stored in a [TLS Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets).
  Update the namespace value in below export command if Tekton Pipelines is installed in a different namespace.
   ```sh
   export NAMESPACE="tekton-pipelines"
   # Generate new self-signed cert.
   openssl req -x509 \
   -newkey rsa:4096 \
   -keyout key.pem \
   -out cert.pem \
   -days 365 \
   -nodes \
   -subj "/CN=tekton-results-api-service.${NAMESPACE}.svc.cluster.local" \
   -addext "subjectAltName = DNS:tekton-results-api-service.${NAMESPACE}.svc.cluster.local"
   # Create new TLS Secret from cert.
   kubectl create secret tls -n ${NAMESPACE} tekton-results-tls \
   --cert=cert.pem \
   --key=key.pem
   ```
- Create PVC if using PVC for logging
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

- Once the secrets are created create a TektonResult CR (Check ##Properties) as below.
  ```sh
  kubectl apply -f config/crs/kubernetes/result/operator_v1alpha1_result_cr.yaml
  ```
- Check the status of installation using following command
  ```sh
  kubectl get tektonresults.operator.tekton.dev
  ```

## Properties
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
  db_sslmode: false
  db_enable_auto_migration: true
  log_level: debug
  logs_api: true
  logs_type: File
  logs_buffer_size: 90kb
  logs_path: /logs
  tls_hostname_override: localhost
  auth_disable: true
  logging_pvc_name: tekton-logs
  secret_name: # optional
  gcs_creds_secret_name: <value>
  gcc_creds_secret_key: <value>
  gcs_bucket_name: <value>
  is_external_db: false
```

These properties are analogous to the one in configmap of tekton results api `tekton-results-api-config` documented at [api.md]:https://github.com/tektoncd/results/blob/4472848a0fb7c1473cfca8b647553170efac78a1/cmd/api/README.md


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
