# Development Guide

## Development Prerequisites
1. [`go`](https://golang.org/doc/install)
1. [`git`](https://help.github.com/articles/set-up-git/)
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
1. [`ko`](https://github.com/google/ko)
1. [`kustomize`](https://github.com/kubernetes-sigs/kustomize)


## Running Codegen

If the files in `pkg/apis` are updated we need to run `codegen` scripts

```shell script
./hack/update-codegen.sh
```

## Running Operator (Development)

### Reset (Clean) Cluster

**Target: Kubernetes**
```shell script
    make clean
```

**Target Openshift**
```shell script
    make TARGET=openshift clean
```

### Setup
- Set `KO_DOCKER_ENV` environment variable ([ko#usage](https://github.com/google/ko#usage))

### Run operator

**Target: Kubernetes**
```shell script
    make apply
```

**Target Openshift**
```shell script
    make TARGET=openshift apply
```
### Install Tekton components
Operator provides an option to choose which components needs to be installed by specifying `profile`.

`profile` is an optional field and supported `profile` are
* **basic**
* **default**
* **all**

1. If profile is `basic` **TektonPipeline** will be installed
1. If profile is `default` or `" "` **TektonPipeline** and **TektonTrigger** will be installed
1. If profile is `all` then all the Tekton Components installed

To create Tekton Components run
```shell script
make apply-cr
make CR=config/basic apply-cr
```
To delete installed Tekton Components run
```shell script
make clean-cr
make CR=config/basic clean-cr
```

## Running Tests

[test docs](../test/README.md)
