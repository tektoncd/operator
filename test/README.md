# Tests

## Run E2E Tests Locally

To run run e2e tests locally,

set environment variables:
- `E2E_DEBUG=on`
- `KO_DOCKER_REPO=<container image registry>`

```shell script
export E2E_DEBUG=on
export KO_DOCKER_REPO=<container image registry>
```
Then run:

```shell script
./test/e2e-tests.sh
```
## Running TESTS

if a cluster is already setup, then set `E2E_DEBUG` variable to skip initialization of a GKE cluster.

```shell script
export E2E_DEBUG=on
```

## Running e2e Tests on Kubernetes

```shell script
    ./test/e2e-tests.sh
```

## Running e2e Tests on Openshift

```shell script
    TARGET=openshift ./test/e2e-tests.sh
```
