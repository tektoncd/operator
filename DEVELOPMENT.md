# Development Guide

## Before you start

The Tekton Operator installs and manages the lifecycle of Tekton components
(Pipelines, Triggers, Chains, Results, Dashboard, Pipelines as Code and more) on
Kubernetes and OpenShift through a set of custom resources. For an overview of what
it manages and how it is structured, see [docs/TektonOperator.md](docs/TektonOperator.md).

Install these tools before you begin:

| Tool | Why | Check |
|------|-----|-------|
| [`go`](https://go.dev/doc/install) | Builds the operator (see [go.mod](go.mod#L3) for the minimum version) | `go version` |
| [`git`](https://git-scm.com/) | Source control | `git --version` |
| [`kubectl`](https://kubernetes.io/docs/tasks/tools/) | Talks to the cluster | `kubectl version --client` |
| [`ko`](https://ko.build/) | Builds and deploys the operator images | `ko version` |
| [`kustomize`](https://github.com/kubernetes-sigs/kustomize) | Renders the deployment manifests | `kustomize version` |
| [`kind`](https://kind.sigs.k8s.io/) | Runs the local development cluster | `kind version` |
| [`docker`](https://www.docker.com/) or [`podman`](https://podman.io/) | Container runtime | `docker version` / `podman version` |

`ko` and `kustomize` are installed automatically into `.bin/` by the `make` targets if
they are not already on your `PATH`.

## Quick start with kind

This is the supported and tested development path. It gives you a local `kind` cluster,
a local image registry and a running operator in a few commands.

1. Fork this repository and clone your fork, then add the upstream remote so you can
   sync later:

   ```bash
   git clone git@github.com:${YOUR_GITHUB_USERNAME}/operator.git
   cd operator
   git remote add upstream git@github.com:tektoncd/operator.git
   git remote set-url --push upstream no_push
   ```

2. Point `ko` at the local registry that `make dev-setup` creates:

   ```bash
   export KO_DOCKER_REPO="localhost:5000"
   ```

3. Create the cluster, the registry and deploy the operator:

   ```bash
   make dev-setup
   ```

   `make dev-setup` creates a `kind` cluster with a local registry and then runs
   `make apply` for you, so the operator is deployed by the time it finishes. It prints
   the location of the generated kubeconfig at the end.

4. Point `kubectl` at the new cluster:

   ```bash
   export KUBECONFIG="${HOME}/.kube/config.kind"
   ```

5. Install the Tekton components by creating a `TektonConfig` custom resource:

   ```bash
   make apply-cr
   ```

6. Verify it worked. The operator pods should be `Running` in the `tekton-operator`
   namespace, the `TektonConfig` should reconcile, and the components should come up in
   `tekton-pipelines`:

   ```bash
   kubectl get pods -n tekton-operator
   kubectl get tektonconfig
   kubectl get pods -n tekton-pipelines
   ```

_Ports used by the `kind` cluster (docker runtime):_

- `8443` - cluster API access
- `80` - ingress http
- `443` - ingress https

If a step fails, see [Troubleshooting](#troubleshooting).

## The development loop

After you change code, rebuild and redeploy the operator with:

```bash
make apply
```

`make apply` uses `ko` to build images, push them to `KO_DOCKER_REPO` and apply the
manifests. To build and load images locally without pushing to a registry, add
`KO_FLAGS=--local`:

```bash
make KO_FLAGS=--local apply
```

### Choosing which components to install

`make apply-cr` creates the `TektonConfig` CR that tells the operator which components
to install, selected by `profile`:

- `lite` — installs TektonPipeline
- `basic` — installs TektonPipeline and TektonTrigger (the default)
- `all` — installs all Tekton components

Select a profile with the `CR` variable:

```bash
make CR=config/lite apply-cr
make CR=config/basic apply-cr
make CR=config/all apply-cr
```

### Reading operator logs

```bash
kubectl logs -n tekton-operator deploy/tekton-operator -f
```

### Resetting the cluster

Remove the installed components, then the operator:

```bash
make clean-cr
make clean
```

## Testing

The PR template asks you to run `make test lint` before submitting. Both run without a
cluster.

- Unit tests:

  ```bash
  make test
  ```

- Linters (`golangci-lint` and `yamllint`):

  ```bash
  make lint
  ```

  To lint a single Go package, pass `PKG`, for example
  `make lint-go PKG=./pkg/reconciler/kubernetes/tektonpipeline/...`.

- End-to-end tests require a live cluster and take longer to run. See the
  [test documentation](test/README.md) for how to run them and what they need.

## Code generation

Run code generation whenever you change the API types under `pkg/apis`:

```bash
./hack/update-codegen.sh
```

This regenerates the deepcopy functions and the typed client, informers and listers for
the operator API group. Never hand-edit generated files — rerun the script instead, and
commit the results alongside your API change.

### Updating dependencies

After changing `go.mod`, refresh the vendored dependencies:

```bash
./hack/update-deps.sh
```

## Developing for OpenShift

The same targets work against OpenShift by setting `TARGET=openshift`:

```bash
make TARGET=openshift apply
make TARGET=openshift clean
```

The OpenShift path deploys OpenShift-specific manifests and components (for example
`TektonAddon`) that do not exist on the Kubernetes path. If you use `podman` as your
container runtime, it is daemonless and needs a user-space socket before you build
images:

```bash
export CONTAINER_RUNTIME=podman
systemctl --user start podman.socket
export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"
```

With the podman runtime the `kind` cluster uses ports `8443` (API), `7080` (ingress
http) and `7443` (ingress https).

## Other clusters

`kind` is the supported, tested development path. Any conformant cluster also works if it
meets the requirements:

- Kubernetes 1.28 or newer (see the compatibility matrix in the [README](README.md#in-support))
- `cluster-admin` privileges
- enough capacity to run the operator and the components you install

Use your provider's own documentation to create the cluster
([GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/creating-a-zonal-cluster),
[EKS](https://docs.aws.amazon.com/eks/latest/userguide/create-cluster.html),
[AKS](https://learn.microsoft.com/azure/aks/learn/quick-kubernetes-deploy-cli)), then
follow [The development loop](#the-development-loop). Detailed, provider-specific
walkthroughs are being moved out of this repo and will be published as dated blog posts
on [tekton.dev](https://tekton.dev/), where they can be updated without a repo change.

## Troubleshooting

- **`KO_DOCKER_REPO` unset or wrong.** `make apply` fails to push images. Export
  `KO_DOCKER_REPO="localhost:5000"` for the local kind registry, or use
  `make KO_FLAGS=--local apply` to skip pushing entirely.
- **Image pull failures against the local registry.** Confirm the registry container is
  running (`docker ps | grep kind-registry`) and reachable at `localhost:5000`. Rerun
  `make dev-setup` if the cluster and registry are out of sync.
- **`podman` socket not running.** Start it with `systemctl --user start podman.socket`
  and export `DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"` before building.
- **Port conflicts.** The kind cluster binds `8443`, and `80`/`443` (docker) or
  `7080`/`7443` (podman). Free the port or stop the conflicting service before running
  `make dev-setup`.
- **Operator pod crashlooping.** Check its logs first:
  `kubectl logs -n tekton-operator deploy/tekton-operator`.
- **Stale CRDs after switching branches.** Run `make clean` followed by `make apply` to
  re-apply the manifests for the current branch.

## Contributing your change

- Read [CONTRIBUTING.md](CONTRIBUTING.md) and the
  [tektoncd/community standards](https://github.com/tektoncd/community/blob/master/standards.md).
- Run `make test lint` and make sure both pass before opening a PR.
- Follow the [commit message guidelines](https://github.com/tektoncd/community/blob/master/standards.md#commit-messages).
- Questions? Reach the maintainers in the `#operator` channel on the
  [Tekton Slack](https://github.com/tektoncd/community/blob/main/contact.md#slack) or at
  the operator working group.

## Additional resources

These help you ramp up on the technology the operator is built on:

- [Tekton Operator concepts](docs/README.md) and [internals](docs/TektonOperator.md)
- The [knative.dev/pkg](https://github.com/knative/pkg) reconciler pattern this operator is built on
- [`ko`](https://ko.build/) for building and deploying Go apps to Kubernetes
- The [Tekton "Hello World" tutorial](https://tekton.dev/docs/getting-started/tasks/)

Ramp up on Custom Resource Definitions (CRDs), which this project uses to extend Kubernetes:

- [The Kubernetes docs on Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) — what "Resource" and "Controller" mean
- [Understanding Kubernetes objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/) — core k8s object model
- [API conventions — types (kinds)](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#types-kinds) — "Objects" and "Lists"
- [Extend the Kubernetes API with CustomResourceDefinitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) — hands-on CRD tutorial

Ramp up on Tekton and Kubernetes operators:

- [Tekton Pipelines README](https://github.com/tektoncd/pipeline/blob/main/docs/README.md) and [installation docs](https://github.com/tektoncd/pipeline/blob/main/docs/install.md)
- [Operator Getting Started](https://operatorhub.io/getting-started)
