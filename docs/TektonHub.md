# Tekton Hub

TektonHub custom resource allows user to install and manage [Tekton Hub][hub].

TektonHub is an optional component and currently cannot be installed through TektonConfig. It has to be installed seperately. It is available for both Kubernetes and OpenShift platform.

To install Tekton Hub on your cluster follow steps as given below:

1.  Create the secrets for the API before we install Tekton Hub. By default,
    Tekton Hub expects this secret to have the following properties:

    - namespace: TargetNamespace defined in TektonHub CR at the time of applying. If
      nothing is specified then based on platform create the secrets. `openshift-pipelines` in case
      of OpenShift, `tekton-pipelines` in case of Kubernetes.
    - name: `api`
    - contains the fields:

      - `GH_CLIENT_ID=<github-client-id>`
      - `GH_CLIENT_SECRET=<github-client-secret>`
      - `GL_CLIENT_ID=<gitlab-client-id>`
      - `GL_CLIENT_SECRET=<gitlab-client-secret>`
      - `BB_CLIENT_ID=<bitbucket-client-id>`
      - `BB_CLIENT_SECRET=<bitbucket-client-secret>`
      - `JWT_SIGNING_KEY=<jwt-signing-key>`
      - `ACCESS_JWT_EXPIRES_IN=<time(eg 30d)>`
      - `REFRESH_JWT_EXPIRES_IN=<time(eg 30d)>`
      - `GHE_URL=<github enterprise url(leave it blank if not using github enterprise>`
      - `GLE_URL=<gitlab enterprise url(leave it blank if not using gitlab enterprise>`

    > _Note 1_: For more details please refer to [here](https://github.com/tektoncd/hub/blob/main/docs/DEPLOYMENT.md#create-git-oauth-applications)

1.  Once the secrets are created now we need to understand how TektonHub CR looks.

    ```yaml
    apiVersion: operator.tekton.dev/v1alpha1
    kind: TektonHub
    metadata:
      name: hub
    spec:
      targetNamespace:
      # <namespace> in which you want to install Tekton Hub. Leave it blank if in case you want to install
      # in default installation namespace ie `openshift-pipelines` in case of OpenShift and `tekton-pipelines` in case of Kubernetes
      api:
        hubConfigUrl: https://raw.githubusercontent.com/tektoncd/hub/main/config.yaml
    ```

    ### API

    The following field helps to configure the API deployment. Provided fields are:

    - `hubConfigUrl`: The place of Tekton Hub config url as shown above.

1.  After configuring the TektonHub spec you can install Tekton Hub by running the command

    ```sh
    kubectl apply -f <name>.yaml
    ```

1.  Check the status of installation using following command

    ```sh
    $ kubectl get tektonhub.operator.tekton.dev
    NAME   VERSION   READY   REASON   APIURL
    hub    v1.6.0    True             https://api.route.url
    ```

[hub]: https://github.com/tektoncd/hub
