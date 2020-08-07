# Development Guide

## Development Prerequisites
1. [`go`](https://golang.org/doc/install): The language Tektoncd-pipeline-operator is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster
1. [operator-sdk v0.17.0](https://github.com/operator-framework/operator-sdk)
1. [ko](https://github.com/google/ko#installation)


## Running Operator Locally (Development)

1. Apply Operator CRD

    `kubectl apply -f config/crds/*_crd.yaml`

1. start operator

    `make local-dev`

1. Update the dependencies

    `make update-deps`

## Running E2E Tests Locally (Development)

1. run

    `make local-test-e2e`

1. to watch resources getting created/deleted, run in a separate terminal:

    `watch -d -n 1 kubectl get all -n tekton-pipelines`

## KO based development workflow

1. Set `KO_DOCKER_ENV` environment variable ([ko#usage](https://github.com/google/ko#usage))

1. Set `KO_DATA_PATH=${GOPATH}/src/github.com/tektoncd/operator/cmd/manager/kodata`

1. run `make ko-apply`
