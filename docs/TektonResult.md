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
        - `user=root`
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
  db_user: test
  db_password: pass
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
  s3_bucket_name: test
  s3_endpoint: aws.com
  s3_hostname_immutable: sdf
  s3_region: west
  s3_access_key_id: 123r
  s3_secret_access_key: sdfjg
  s3_multi_part_size: 888mb
  logging_pvc_name: tekton-logs
```

These properties are analogous to the one in configmap of tekton results api `tekton-results-api-config` documented at [api.md]:https://github.com/tektoncd/results/blob/4472848a0fb7c1473cfca8b647553170efac78a1/cmd/api/README.md


[result]:https://github.com/tektoncd/results

