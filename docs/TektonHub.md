# Tekton Hub

TektonHub custom resource allows user to install and manage [Tekton Hub][hub].

TektonHub is an optional component and currently cannot be installed through TektonConfig. It has to be installed
seperately. It is available for both Kubernetes and OpenShift platform.

To install Tekton Hub on your cluster follow steps as given below:


  **Note** :
  - Initially user had to enable Tekton Hub Authentication, get the Hub token with required scopes and then hit the catalog refresh api to load the resources in the database.
  - Now user doesn't needs to enable login mechanism to get the resources populated in the database as resources will be automatically populated in the database when the api is up and running
  - Resources in the hub db will be also automatically refreshed with the updated data with the time which is specified in the Hub CR i.e `catalogRefreshInterval: 30m`. Default time interval is 30m
  -  If you are using your database instead of default one then
  secret name for the database should be `tekton-hub-db` and you need to create the secret in your targetNamespace with the following keys
  - **The dependency of config.yaml from git is removed and complete config data is moved into API configMap. Now user can add the config data i.e.
    `categories, catalogs, scopes and defaultScopes` in the Hub CR itself. If user does not add any data then default
    data provided by Hub in the API configMap will be used**

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: tekton-hub-db
    labels:
      app: tekton-hub-db
  type: Opaque
  stringData:
    POSTGRES_HOST: <The name of the host of the database>
    POSTGRES_DB: <Name of the database>
    POSTGRES_USER: <The name of user account>
    POSTGRES_PASSWORD: <The password of user account>
    POSTGRES_PORT: <The port that the database is listening on>
  ```

### 1. Without login and rating

- This is how TektonHub CR looks

  ```yaml
  apiVersion: operator.tekton.dev/v1alpha1
  kind: TektonHub
  metadata:
    name: hub
  spec:
    targetNamespace:
    # <namespace> in which you want to install Tekton Hub. Leave it blank if in case you want to install
    # in default installation namespace ie `openshift-pipelines` in case of OpenShift and `tekton-pipelines` in case of Kubernetes
    db:                      # 👈 Optional: If user wants to use his database
      secret: tekton-hub-db  # 👈 Name of db secret should be `tekton-hub-db`
  
    categories:                     # 👈 Optional: If user wants to use his categories 
      - Automation
      - Build Tools
      - CLI
      - Cloud
      - Code Quality
      - ...
  
    catalogs:                       # 👈 Optional: If user wants to use his catalogs
      - name: tekton
        org: tektoncd
        type: community
        provider: github
        url: https://github.com/tektoncd/catalog
        revision: main

    scopes:                         # 👈 Optional: User can define his scopes  
      - name: agent:create
        users: [abc, qwe, pqr]
      - name: catalog:refresh
        users: [abc, qwe, pqr]
      - name: config:refresh
        users: [abc, qwe, pqr]

    default:                       # 👈 Optional: User can define his default scopes
      scopes:
        - rating:read
        - rating:write

    api:
      catalogRefreshInterval: 30m     # After every 30min catalog resources in the hub db would be refreshed to get the updated data from the catalog. Supported time units are As(A seconds), Bm(B minutes) Ch(C hours), Dd(D days) and Ew(E weeks).
  ```

- You can install Tekton Hub by running the command

    ```sh
    kubectl apply -f <name>.yaml
    ```

- Check the status of installation using following command

  ```sh
  $ kubectl get tektonhub.operator.tekton.dev
  NAME   VERSION   READY   REASON   APIURL                  UIURL
  hub    v1.8.0    True             https://api.route.url   https://ui.route.url
  ```

### 2. With login and rating
1. Create the secrets for the API before we install Tekton Hub. By default, Tekton Hub expects this secret to have the
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
      db:                      # 👈 Optional: If user wants to use his database
        secret: tekton-hub-db  # 👈 Name of db secret should be `tekton-hub-db`
   
      categories:                     # 👈 Optional: If user wants to use his categories 
        - Automation
        - Build Tools
        - CLI
        - Cloud
        - Code Quality
        - ...
  
      catalogs:                       # 👈 Optional: If user wants to use his catalogs
        - name: tekton
          org: tektoncd
          type: community
          provider: github
          url: https://github.com/tektoncd/catalog
          revision: main

      scopes:                         # 👈 Optional: User can define his scopes 
        - name: agent:create
          users: [abc, qwe, pqr]
        - name: catalog:refresh
          users: [abc, qwe, pqr]
        - name: config:refresh
          users: [abc, qwe, pqr]

      default:                       # 👈 Optional: User can define his default scopes
        scopes:
          - rating:read
          - rating:write
   
      api:
        catalogRefreshInterval: 30m     # After every 30min catalog resources in the hub db would be refreshed to get the updated data from the catalog. Supported time units are As(A seconds), Bm(B minutes) Ch(C hours), Dd(D days) and Ew(E weeks).
    ```

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
