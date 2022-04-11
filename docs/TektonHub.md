# Tekton Hub

TektonHub custom resource allows user to install and manage [Tekton Hub][hub].

TektonHub is an optional component and currently cannot be installed through TektonConfig. It has to be installed
seperately. It is available for both Kubernetes and OpenShift platform.

To install Tekton Hub on your cluster follow steps as given below:

1. Update the [config.yaml](https://raw.githubusercontent.com/tektoncd/hub/master/config.yaml) to add one user with all
   the scopes such as
    - refresh a catalog
    - refresh config file
    - create an agent token

   **NOTE**:- You can maintain [config.yaml](https://raw.githubusercontent.com/tektoncd/hub/master/config.yaml) in any
   repo but make sure it is accessible to the Hub API server or you can either fork the project [Tekton Hub][hub]

   For example:
    ```
   scopes:
   - name: agent:create
     users: [foo]        <<< Where `foo` is your Github Handle
   - name: catalog:refresh
     users: [foo]
   - name: config:refresh
     users: [foo]
      ```

   **NOTE**:-
    - With the `catalog:refresh` scope user will be able to refresh the catalog and all the resources in db. For more
      details refere to [here](https://github.com/tektoncd/hub/blob/main/docs/DEPLOYMENT.md#add-resources-in-db)
    - With the `agent:create` scope user can setup a cronjob which will refresh your db after an interval if there are
      any changes in your catalog. For more details
      refer [here](https://github.com/tektoncd/hub/blob/main/docs/DEPLOYMENT.md#setup-catalog-refresh-cronjob-optional)
    - With the `config:refresh` scope user can get additional scopes. For more details
      refer [here](https://github.com/tektoncd/hub/blob/main/docs/DEPLOYMENT.md#setup-catalog-refresh-cronjob-optional)

- Commit the changes and push the changes to your fork

2. Create the secrets for the API before we install Tekton Hub. By default, Tekton Hub expects this secret to have the
   following properties:

    - namespace: TargetNamespace defined in TektonHub CR at the time of applying. If nothing is specified then based on
      platform create the secrets. `openshift-pipelines` in case of OpenShift, `tekton-pipelines` in case of Kubernetes.
    - name: `tekton-hub-api`
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

3. Once the secrets are created now we need to understand how TektonHub CR looks.

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
        hubConfigUrl: https://raw.githubusercontent.com/tektoncd/hub/main/config.yaml # 👈 MUST: Change the file URL here to point to your fork
    ```

   ### API

   The following field helps to configure the API deployment. Provided fields are:

    - `hubConfigUrl`: The place of Tekton Hub config url as shown above.

4. After configuring the TektonHub spec you can install Tekton Hub by running the command

    ```sh
    kubectl apply -f <name>.yaml
    ```

5. Check the status of installation using following command

    ```sh
    $ kubectl get tektonhub.operator.tekton.dev
    NAME   VERSION   READY   REASON   APIURL                  UIURL
    hub    v1.6.0    True             https://api.route.url   https://ui.route.url
    ```

[hub]: https://github.com/tektoncd/hub
