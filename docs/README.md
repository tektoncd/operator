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

## Running Tests

[test docs](../test/README.md)
