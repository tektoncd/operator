# Development Guide

## Development Prerequisites
1. [`go`](https://golang.org/doc/install)
1. [`git`](https://help.github.com/articles/set-up-git/)
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
1. [`ko`](https://github.com/google/ko)
1. [`kustomize`](https://github.com/kubernetes-sigs/kustomize)

## Getting started

- [Development Guide](#development-guide)
  - [Development Prerequisites](#development-prerequisites)
  - [Getting started](#getting-started)
    - [Ramp up](#ramp-up)
      - [Ramp up on CRDs](#ramp-up-on-crds)
      - [Ramp up on Tekton Pipelines](#ramp-up-on-tekton-pipelines)
      - [Ramp up on Kubernetes Operators](#ramp-up-on-kubernetes-operators)
    - [Checkout your fork](#checkout-your-fork)
    - [Requirements](#requirements)
  - [Kubernetes cluster](#kubernetes-cluster)
  - [Environment Setup](#environment-setup)
  - [Iterating](#iterating)
    - [Install Operator](#install-operator)
  - [Accessing logs](#accessing-logs)
  - [Updating the clustertasks in OpenShift addons](#updating-the-clustertasks-in-openshift-addons)
  - [Running Codegen](#running-codegen)
  - [Setup development environment on localhost](#setup-development-environment-on-localhost)
    - [Pre-requests](#pre-requests)
    - [setup with docker runtime](#setup-with-docker-runtime)
    - [setup with podman runtime](#setup-with-podman-runtime)
  - [Running Operator (Development)](#running-operator-development)
    - [Reset (Clean) Cluster](#reset-clean-cluster)
    - [Setup](#setup)
    - [Run operator](#run-operator)
    - [Install Tekton components](#install-tekton-components)
  - [Running Tests](#running-tests)

### Ramp up

Welcome to the project!! You may find these resources helpful to ramp up on some
of the technology this project is built on.

#### Ramp up on CRDs

This project extends Kubernetes (aka
`k8s`) with Custom Resource Definitions (CRDSs). To find out more:

- [The Kubernetes docs on Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) -
  These will orient you on what words like "Resource" and "Controller"
  concretely mean
- [Understanding Kubernetes objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/) -
  This will further solidify k8s nomenclature
- [API conventions - Types(kinds)](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#types-kinds) -
  Another useful set of words describing words. "Objects" and "Lists" in k8s
  land
- [Extend the Kubernetes API with CustomResourceDefinitions](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/)-
  A tutorial demonstrating how a Custom Resource Definition can be added to
  Kubernetes without anything actually "happening" beyond being able to list
  Objects of that kind

#### Ramp up on Tekton Pipelines

- [Tekton Pipelines README](https://github.com/tektoncd/pipeline/blob/master/docs/README.md) -
  Some of the terms here may make more sense!
- Install via
  [official installation docs](https://github.com/tektoncd/pipeline/blob/master/docs/install.md)
  or continue though [getting started for development](#getting-started)
- [Tekton Pipeline "Hello World" tutorial](https://github.com/tektoncd/pipeline/blob/master/docs/tutorial.md) -
  Define `Tasks`, `Pipelines`, and `PipelineResources`, see what happens when
  they are run

#### Ramp up on Kubernetes Operators

- [Operator Getting Started](https://operatorhub.io/getting-started)

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/tektoncd/operator` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/tektoncd
cd ${GOPATH}/src/github.com/tektoncd
git clone git@github.com:${YOUR_GITHUB_USERNAME}/operator.git
cd operator
git remote add upstream git@github.com:tektoncd/operator.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

### Requirements

You must install these tools:

1. [`go`](https://golang.org/doc/install): The language Tekton Pipelines is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`dep`](https://github.com/golang/dep): For managing external Go
   dependencies. - Please Install dep v0.5.0 or greater.
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster

Your [`$GOPATH`] setting is critical for `go` to function properly.

## Kubernetes cluster

Docker for Desktop using an edge version has been proven to work for both
developing and running Pipelines. The recommended configuration is:

- Kubernetes version 1.11 or later
- 4 vCPU nodes (`n1-standard-4`)
- Node autoscaling, up to 3 nodes
- API scopes for cloud-platform

To setup a cluster with GKE:

1. [Install required tools and setup GCP project](https://github.com/knative/docs/blob/master/docs/install/Knative-with-GKE.md#before-you-begin)
   (You may find it useful to save the ID of the project in an environment
   variable (e.g. `PROJECT_ID`).

1. Create a GKE cluster (with `--cluster-version=latest` but you can use any
   version 1.11 or later):

   ```bash
   export PROJECT_ID=my-gcp-project
   export CLUSTER_NAME=mycoolcluster

   gcloud container clusters create $CLUSTER_NAME \
    --enable-autoscaling \
    --min-nodes=1 \
    --max-nodes=3 \
    --scopes=cloud-platform \
    --enable-basic-auth \
    --no-issue-client-certificate \
    --project=$PROJECT_ID \
    --region=us-central1 \
    --machine-type=n1-standard-4 \
    --image-type=cos \
    --num-nodes=1 \
    --cluster-version=latest
   ```

   Note that
   [the `--scopes` argument to `gcloud container cluster create`](https://cloud.google.com/sdk/gcloud/reference/container/clusters/create#--scopes)
   controls what GCP resources the cluster's default service account has access
   to; for example to give the default service account full access to your GCR
   registry, you can add `storage-full` to your `--scopes` arg.

1. Grant cluster-admin permissions to the current user:

   ```bash
   kubectl create clusterrolebinding cluster-admin-binding \
   --clusterrole=cluster-admin \
   --user=$(gcloud config get-value core/account)
   ```

## Environment Setup

To [run/test your operator](#install-operator) you'll need to set these
environment variables (we recommend adding them to your `.bashrc`):

1. `GOPATH`: If you don't have one, simply pick a directory and add
   `export GOPATH=...`
1. `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.

`.bashrc` example:

```shell
export GOPATH="$HOME/go"
export PATH="${PATH}:${GOPATH}/bin"
```

## Iterating

While iterating on the project, you may need to:

1. [Install/Run Operator](#install-operator)
1. Verify it's working by [looking at the logs](#accessing-logs)
1. Update your (external) dependencies with: `./hack/update-deps.sh`.

   **Running dep ensure manually, will pull a bunch of scripts deleted
   [here](./hack/update-deps.sh#L29)**

1. Update your type definitions with: `./hack/update-codegen.sh`.
1. [Add new CRD types](#adding-new-types)
1. [Add and run tests](./test/README.md#tests)

### Install Operator

**Note: this needs to be completed! We don't yet have any code or config to deploy,
watch this space!**

## Accessing logs

**Note: this needs to be completed! We don't yet have any code or config to deploy,
watch this space!**

## Updating the clustertasks in OpenShift addons

You can update the clustertasks present in the codebase with the latest using the script present at `/hack/openshift/update-tasks.sh`

You can edit the script to mention the specific version of the task or to add a new task.

Then all the tasks mentioned in the script can be added to codebase using

```shell
./hack/openshift/fetch-tektoncd-catalog-tasks.sh cmd/openshift/operator/kodata/tekton-addon/addons/02-clustertasks/source_external
```
## Running Codegen

If the files in `pkg/apis` are updated we need to run `codegen` scripts

```shell script
./hack/update-codegen.sh
```

## Setup development environment on localhost
Here are the steps to setup development environment on your localhost with local registry

### Pre-requests
   - either `docker` or `podman` runtime
   - [kind](https://github.com/kubernetes-sigs/kind)

### setup with docker runtime
```bash
export KO_DOCKER_REPO="localhost:5000"

make dev-setup
```
kubernetes cluster ports used
* `8443` - cluster api access
* `80` - ingress http
* `443` - ingress https

### setup with podman runtime
`podman` is a daemonless container engine. You have to setup a socket service on user space.
```bash
$ export KO_DOCKER_REPO="localhost:5000"
$ export CONTAINER_RUNTIME=podman
$ systemctl --user start podman.socket
$ export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock

$ make dev-setup
```
kubernetes cluster ports used
* `8443` - cluster api access
* `7080` - ingress http
* `7443` - ingress https


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
- Set `KO_DOCKER_REPO` environment variable ([ko#usage](https://github.com/google/ko#usage))
- If you want to use local image rather than pushing image to registry you can set flags with `KO_FLAGS=--local` when you run operator

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
* **lite**
* **basic**
* **all**

1. If profile is `lite` **TektonPipeline** will be installed
1. If profile is `basic` **TektonPipeline** and **TektonTrigger** will be installed
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

[test docs](test/README.md)
