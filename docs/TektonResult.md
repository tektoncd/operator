# Tekton Result

TektonResult custom resource allows user to install and manage [Tekton Result][result].

TektonResult is an optional component and currently cannot be installed through TektonConfig. It has to be installed seperately.

NOTE: TektonResult is enabled only on Kubernetes Platform and not on OpenShift.

To install Tekton Result on your cluster follow steps as given below:
- Make sure Tekton Pipelines is installed on your cluster, using the Operator.
- Generate a database root password.
  A database root password must be generated and stored in a [Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/)
  before installing results. By default, Tekton Results expects this secret to have
  the following properties:

    - namespace: `tekton-pipelines`
    - name: `tekton-results-mysql`
    - contains the fields:
        - `user=root`
        - `password=<your password>`

  If you are not using a particular password management strategy, the following
  command will generate a random password for you:
  Update namespace value in the command if Tekton Pipelines is installed in a different namespace..

   ```sh
   $ kubectl create secret generic tekton-results-mysql --namespace=tekton-pipelines --from-literal=user=root --from-literal=password=$(openssl rand -base64 20)
   ```
- Generate cert/key pair. 
  Note: Feel free to use any cert management software to do this!

  Tekton Results expects the cert/key pair to be stored in a [TLS Kubernetes Secret](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets).
  Update the namespace value in below export command if Tekton Pipelines is installed in a different namespace.
   ```sh
   # Generate new self-signed cert.
   $ export NAMESPACE="tekton-pipelines"
   $ openssl req -x509 \
   -newkey rsa:4096 \
   -keyout key.pem \
   -out cert.pem \
   -days 365 \
   -nodes \
   -subj "/CN=tekton-results-api-service.${NAMESPACE}.svc.cluster.local" \
   -addext "subjectAltName = DNS:tekton-results-api-service.${NAMESPACE}.svc.cluster.local"
   # Create new TLS Secret from cert.
   $ kubectl create secret tls -n ${NAMESPACE} tekton-results-tls \
   --cert=cert.pem \
   --key=key.pem
   ```
- Once the secrets are created create a TektonResult CR as below.
  ```sh
  kubectl apply -f config/crs/kubernetes/result/operator_v1alpha1_result_cr.yaml
  ```
- Check the status of installation using following command
  ```sh
  kubectl get tektonresults.operator.tekton.dev
  ```

[result]:https://github.com/tektoncd/results