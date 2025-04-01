<!--
---
linkTitle: "TektonChain"
weight: 9
---
-->
# Tekton Chain

TektonChain custom resource allows user to install and manage [Tekton Chains][chains].

It is recommended to install the component through [TektonConfig](./TektonConfig.md).

- TektonChain CR is as below

    - On Kubernetes, TektonChain CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonChain
    metadata:
      name: chain
    spec:
      disabled: false
      targetNamespace: tekton-pipelines
    ```

    - On OpenShift, TektonChain CR is as below:

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonChain
    metadata:
      name: chain
    spec:
      disabled: false
      targetNamespace: openshift-pipelines
    ```

- Check the status of installation using following command:

    ```sh
    kubectl get tektonchains.operator.tekton.dev
    ```

## Chain Config

There are some fields which you can define on Tekton Chains CR to configure the behaviour of the chains
If nothing is set, the operator sets these default properties:
  artifacts.taskrun.format: in-toto
  artifacts.taskrun.storage: oci
  artifacts.oci.storage: oci
  artifacts.oci.format: simplesigning
  artifacts.pipelinerun.format: in-toto
  artifacts.pipelinerun.storage: oci


### Properties (Mandatory)

 - `targetNamespace`

    Setting this field to provide the namespace in which you want to install the chains component.

### Properties (Optional)

These fields don't have default values so will be considered only if user passes them. By default, Operator won't add
these fields in CR and won't configure for chains.

The Default values for some of these fields are already set in chains and are not set by Operator. If user passes some
values then those will be set for the particular field.

Details of the field can be found in [Tekton Chains Config][chains-config]

Chains CR will look like this after providing all the fields.
```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonChain
metadata:
  name: chain
spec:
  disabled: false
  targetNamespace: tekton-pipelines
  generateSigningSecret: true # default value: false
  controllerEnvs:
    - name: MONGO_SERVER_URL      # This is the only field supported at the moment which is optional and when added by user, it is added as env to Chains controller
      value: #value               # This can be provided same as env field of container
  artifacts.taskrun.format: in-toto # default value: in-toto
  artifacts.taskrun.storage: tekton,oci (comma separated values) # default value: oci
  artifacts.taskrun.signer: x509
  artifacts.oci.storage: oci (comma separated values)
  artifacts.oci.format: simplesigning
  artifacts.oci.signer: x509
  artifacts.pipelinerun.format: in-toto # default value: in-toto
  artifacts.pipelinerun.storage: tekton,oci (comma separated values) # default value: oci
  artifacts.pipelinerun.signer: x509
  artifacts.pipelinerun.enable-deep-inspection: #value (boolean - true/false)
  storage.gcs.bucket: #value
  storage.oci.repository: #value
  storage.oci.repository.insecure: #value (boolean - true/false)
  storage.docdb.url: #value
  storage.docdb.mongo-server-url: #value
  storage.docdb.mongo-server-url-dir: #value
  storage.grafeas.projectid: #value
  storage.grafeas.noteid: #value
  storage.grafeas.notehint: #value
  builder.id: #value
  builddefinition.buildtype: #value
  signers.x509.fulcio.enabled: #value (boolean - true/false)
  signers.x509.fulcio.address: #value
  signers.x509.fulcio.issuer: #value
  signers.x509.fulcio.provider: #value
  signers.x509.identity.token.file: #value
  signers.x509.tuf.mirror.url: #value
  signers.kms.kmsref: #value
  signers.kms.auth.address: #value
  signers.kms.auth.token: #value
  signers.kms.auth.token-path: #value
  signers.kms.auth.oidc.path: #value
  signers.kms.auth.oidc.role: #value
  signers.kms.auth.spire.sock: #value
  signers.kms.auth.spire.audience: #value
  transparency.enabled: #value (boolean - true/false)
  transparency.url: #value
```
- `disabled` : if the value set as `true`, chains feature will be disabled (default: `false`)
- `generateSigningSecret`: When set to true, the operator will generate a cosign key pair (`cosign.key` as the  private key, `cosign.password` as the password for decrypting private key and `cosign.pub` as the public key) and store them in the signing-secrets secret within the tekton-pipelines namespace. This secret is used by the Chains controller to sign Tekton artifacts (taskruns, pipelineruns).
   If the signing-secret is empty, enabling generateSigningSecret will create a new Cosign key pair and password. However, if the secret already contains data, enabling generateSigningSecret should not overwrite the existing secret. It is important to note that:
 * The user should retrieve and store the `cosign.pub` public key in a secure location verify later artifact attestations.
 * the operator doesnt provide any function about key rotation to limit potential security issues
 * the operator doesnt provide any function for auditing key usage
 * the operator doesnt provide any function for proper access control to the key


[chains]:https://github.com/tektoncd/chains
[chains-config]:https://github.com/tektoncd/chains/blob/main/docs/config.md
