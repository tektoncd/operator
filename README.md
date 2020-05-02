# Tektoncd Operator

Kubernetes [Operator](https://operatorhub.io/getting-started) to manage installations, updates and removal of Tektoncd projects (pipeline, dashboard, …)

The following steps will install [Tekton Pipeline](https://github.com/tektoncd/pipeline) and configure it appropriately for your cluster.
1. Create namespace: `tekton-operator`  

    `kubectl create namespace tekton-operator`
    
2. Apply Operator CRD

    `kubectl apply -f deploy/crds/operator_v1alpha1_config_crd.yaml`  
    `kubectl apply -f deploy/crds/operator_v1alpha1_addon_crd.yaml`
    
3. Deploy the Operator  

    `kubectl -n tekton-operator apply -f deploy/`  
    
    The Operator will automatic install `Tekton pipeline` with `v0.12.0` in the namespace `tekton-pipeline`

## Development Prerequisites
1. [`go`](https://golang.org/doc/install): The language Tektoncd-pipeline-operator is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster
1. operator-sdk: https://github.com/operator-framework/operator-sdk


## Running Operator Locally (Development)

1. Apply Operator CRD

    `kubectl apply -f deploy/crds/*_crd.yaml`

1. start operator

    `make local-dev`

1. Update the dependencies

    `make update-deps`

## Running E2E Tests Locally (Development)

1. run

    `local-test-e2e`

1. to watch resources getting created/deleted, run in a separate terminal:

    `watch -d -n 1 kubectl get all -n tekton-pipelines`

## Building the Operator Image
1. Enable go mod  

    `export GO111MODULE=on`
    
2. Build go and the container image  

    `operator-sdk build ${YOUR_REGISTORY}/openshift-pipelines-operator:${IMAGE_TAG}`
    
3. Push the container image  

    `docker push ${YOUR-REGISTORY}/openshift-pipelines-operator:${IMAGE-TAG}`
    
4. Edit the 'image' value in deploy/operator.yaml to match to your image  

## The CRD
This is a sample of [crd](https://github.com/tektoncd/operator/blob/master/deploy/crds/operator_v1alpha1_config_cr.yaml)
```
apiVersion: operator.tekton.dev/v1alpha1
kind: Config
metadata:
  name: cluster
spec:
  targetNamespace: tekton-pipelines
```
The crd is `Cluster scope`, and `targetNamespace` means `Tekton Pipleine` will installed in it.  

By default the cr will be created automatic, means `Tekton Pipeline` will be installed automatic when Operator installed.
To change the behavior, you could add argument: `no-auto-install=true` to deploy/operator.yaml, like this:  

```
args:
- --no-auto-install=true
```

Then install `Tekton Pipeline` manually:  

`kubectl apply -f deploy/crds/*_cr.yaml`

## Addon components

Supported addon components are installed by creating the 'addon' CR for the component.

Sample CR

```
apiVersion: operator.tekton.dev/v1alpha1
kind: Addon
metadata:
  name: dashboard
spec:
  version: v0.1.0
```

The current supported components and versions are:

- dashboard
  - v0.1.1
  - v0.2.0
  - openshift-v0.2.0
  - v0.6.1
- extensionwebhooks
  - v0.2.0
  - openshift-v0.2.0
  - v0.6.1
- trigger
  - v0.1.0
  - v0.2.1
  - v0.3.1
  - v0.4.0
  - v0.5.0
