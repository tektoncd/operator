# Tests

## Run E2E Tests Locally

To run run e2e tests locally (outside Tektoncd Plumbing CI):

If cluster is already available, then skip creation of GKE cluster by setting
`E2E_SKIP_CLUSTER_CREATION` environment variable:

```bash
export E2E_SKIP_CLUSTER_CREATION=true
```

If operator is already installed. (For example installed using any of the releases for Kubernetes or OpenShift),
then skip operator installation by setting `E2E_SKIP_OPERATOR_INSTALLATION` environment variable:

```bash
export E2E_SKIP_OPERATOR_INSTALLATION=true
```

Else, set `KO_DOCKER_ENV` so that operator resources could be installed before running 
e2e tests.

```bash
unset E2E_SKIP_OPERATOR_INSTALLATION
export KO_DOCKER_REPO=<container image registry>
```

Then run:

```shell script
./test/e2e-tests.sh
```

## Example 1: Testing in dev

if a cluster is already available and if we are testing code in development, then set `E2E_SKIP_CLUSTER_CREATION` variable to skip initialization of a GKE cluster.

```shell script
export E2E_SKIP_CLUSTER_CREATION=true
KO_DOCKER_REPO=<container image registry>
./test/e2e-tests.sh
```

## Example 2: Testing an Operator release for OpenShift

To test an already released version of the operator build for OpenShift Platform, using an
already existing OpenShift Cluster.

Install the operator on OpenShift. Then,

```shell script
E2E_SKIP_CLUSTER_CREATION=true E2E_SKIP_OPERATOR_INSTALLATION=true TARGET=openshift ./test/e2e-tests.sh
```
