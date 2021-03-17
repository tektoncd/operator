# Tekton Operator Development Guide

## Getting started

1. [Ramp up on kubernetes and CRDs](#ramp-up-on-crds)
1. [Ramp Tekton Pipelines](#ramp-up-on-tekton-pipelines)
1. Create [a GitHub account](https://github.com/join)
1. Setup
   [GitHub access via SSH](https://help.github.com/articles/connecting-to-github-with-ssh/)
1. [Create and checkout a repo fork](#checkout-your-fork)
1. Set up your [shell environment](#environment-setup)
1. Install [requirements](#requirements)
1. [Set up a Kubernetes cluster](#kubernetes-cluster)
1. [Configure kubectl to use your cluster](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
1. [Set up a docker repository you can push to](https://github.com/knative/serving/blob/master/docs/setting-up-a-docker-registry.md)
1. [Install Tekton Operator](#install-operator)
1. [Iterate!](#iterating)

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
./hack/openshift/update-tasks.sh release-v0.22 cmd/openshift/operator/kodata/tekton-addon/1.4.0 v0.22.0
```