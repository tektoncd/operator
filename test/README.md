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
