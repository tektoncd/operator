# Development Guide

## Prerequisites

Before you begin, ensure you have the following tools installed:

1. [`Go`](https://go.dev/doc/install): The language the Tekton Operator is built in (1.20 or later recommended)
2. [`git`](https://git-scm.com/): For source control
3. [`ko`](https://ko.build/): For building and deploying Go applications to Kubernetes
4. [`kubectl`](https://kubernetes.io/docs/tasks/tools/): For interacting with the cluster
5. A local Kubernetes environment like [`kind (Kubernetes in Docker)`](https://kind.sigs.k8s.io/) or [`minikube`](https://minikube.sigs.k8s.io/docs/)
6. A container runtime: [`docker`](https://www.docker.com/) or [`podman`](https://podman.io/)

## Local Development Environment (Recommended)

For first-time contributors and daily development, a local `kind` cluster is the fastest and easiest way to test your changes.

### Setting up with Docker runtime

If you are using Docker, set up your local cluster and local registry with the following commands:

```go
export KO_DOCKER_REPO="localhost:5000"
```

```go
make dev-setup
```

_Kubernetes cluster ports used:_

- `8443` - cluster api access

- `80` - ingress http

- `443` - ingress https

### Setting up with Podman runtime

`podman` is a daemonless container engine. You must set up a socket service in user space before creating the cluster:

```go
export KO_DOCKER_REPO="localhost:5000"
export CONTAINER_RUNTIME=podman
systemctl --user start podman.socket
export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
```

```bash
make dev-setup
```

_Kubernetes cluster ports used:_

- `8443` - cluster api access

- `7080` - ingress http

- `7443` - ingress https

## Building and Deploying

The Tekton Operator uses `ko` to build container images and deploy them directly to your cluster.

### Setup `ko`

Ensure your `KO_DOCKER_REPO` environment variable is set (e.g., to your local registry `localhost:5000` created in the previous step). If you want to use a local image rather than pushing the image to a registry, you can set the flags with `export KO_FLAGS=--local`.

### Run the Operator

Target: Kubernetes

```go
make apply
```

Target: OpenShift

```go
make TARGET=openshift apply
```

### Install Tekton components

The Operator provides an option to choose which components need to be installed by specifying a `profile`.

`profile` is an optional field. Supported profiles are:

- `lite`: Installs TektonPipeline

- `basic`: Installs TektonPipeline and TektonTrigger

- `all`: Installs all Tekton Components

To create Tekton Components, run:

```go
make apply-cr
```

Or specify a `profile`:

```go
make CR=config/basic clean-cr
```

To delete installed Tekton Components, run:

```go
make clean-cr
```

Or specify a profile:

```go
make CR=config/basic clean-cr
```

### Reset (Clean) Cluster

To wipe the operator and clean the cluster:

Target: Kubernetes

```go
make clean
```

Target: OpenShift

```go
make TARGET=openshift clean
```

## Iterating and Testing

While iterating on the project, you may need to update your dependencies or generated code:

1. Update your (external) dependencies with: `./hack/update-deps.sh`.

2. Update your type definitions (if files in `pkg/apis` are updated): `./hack/update-codegen.sh`.

### Running Tests

For full documentation on running unit and E2E tests, see the [test documentation](test/README.md).

## Cloud Environments (Advanced)

If you need to test the operator against a specific cloud provider for integration validation, follow the setup instructions below.

**NOTE**: For daily development, the local `kind` cluster detailed above is highly recommended.

### Google Kubernetes Engine (GKE)

To set up a cluster with GKE, you must first install the required CLI tools and configure your Google Cloud project:

#### Prerequisites

1. Install the [`Google Cloud CLI (gcloud)`](https://cloud.google.com/sdk/docs/install)
2. Create a [`Google Cloud Project with billing enabled`](https://docs.cloud.google.com/billing/docs/how-to/modify-project)

#### Environment Setup

Authenticate your local environment and set your target project ID. (You may find it useful to save the ID of the project in an environment variable, e.g., `PROJECT_ID`).

```go
gcloud auth login
export PROJECT_ID="<YOUR_PROJECT_ID>"
export CLUSTER_NAME="tekton-operator-dev"
gcloud config set project $PROJECT_ID
```

#### Create the GKE cluster

Once your tools are configured, run the following script to spin up the GKE cluster.

**NOTE**: This uses the `regular` release channel to ensure a modern, supported Kubernetes version.

```go
gcloud container clusters create $CLUSTER_NAME \
  --enable-autoscaling \
  --min-nodes=1 \
  --max-nodes=3 \
  --enable-basic-auth \
  --no-issue-client-certificate \
  --project=$PROJECT_ID \
  --zone=us-central1 \
  --machine-type=n1-standard-4 \
  --image-type=cos \
  --num-nodes=1 \
  --release-channel=regular \
  --scopes=cloud-platform \
```

**NOTE**: The `--scopes` argument for `gcloud container cluster create` controls what GCP resources the cluster's default service account has access to; for example, to give the default service account full access to your GCR registry, you can add `storage-full` to your `--scopes` arg.

#### Grant Permissions

Grant cluster-admin permissions to the current user so the operator can deploy components:

```go
kubectl create clusterrolebinding cluster-admin-binding \
  --clusterrole=cluster-admin \
  --user=$(gcloud config get-value core/account)
```

## Additional Resources

Welcome to the project! You may find these resources helpful to ramp up on some of the technology this project is built on:

- [`Tekton "Hello World" tutorial`](https://tekton.dev/docs/getting-started/tasks/)
- [`Tekton Operator Concepts`](docs/README.md)
- CRDs: [`The Kubernetes docs on Custom Resources`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
