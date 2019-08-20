# Tektoncd Operator

Kubernetes [Operator](https://operatorhub.io/getting-started) to manage installations, updates and removal of Tektoncd projects (pipeline, dashboard, …)

The following steps will install [Tekton Pipeline](https://github.com/tektoncd/pipeline) and configure it appropriately for your cluster.
1. Create namespace: `tekton-operator`  

    `kubectl create namespace tekton-operator`
    
2. Apply Operator crd  

    `kubectl apply -f deploy/crds/*_crd.yaml`
    
3. Deploy the Operator  

    `kubectl -n tekton-operator apply -f deploy/`  
    
    The Operator will automatic install `Tekton pipeline` with `v0.5.2` in the namespace `tekton-pipeline`

## Development Prerequisites
1. [`go`](https://golang.org/doc/install): The language Tektoncd-pipeline-operator is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster
1. operator-sdk: https://github.com/operator-framework/operator-sdk


## Running Operator Locally

To run the operator lcoally during development:

    `make local-dev`

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
