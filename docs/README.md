# Development Guide

## Development Prerequisites
1. [`go`](https://golang.org/doc/install): The language Tektoncd-pipeline-operator is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster
1. [operator-sdk v0.17.0](https://github.com/operator-framework/operator-sdk)
1. [ko](https://github.com/google/ko#installation)

## Running Codegen

If the files in `pkg/apis` are updated we need to run `codegen` scripts

```shell script
./hack/update-codegen.sh
```

## Running Operator (Development)

1. Set `KO_DOCKER_ENV` environment variable ([ko#usage](https://github.com/google/ko#usage))

1. Set `KO_DATA_PATH=${GOPATH}/src/github.com/tektoncd/operator/cmd/operator/kodata`

1. run `make ko-apply`
