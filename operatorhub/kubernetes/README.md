# How to generate Operator Bundles for Kubernetes

For Kubernetes Platform ([operatorhub.io](https://operatorhub.io)) release we generate the
bundle for using `release-manifest` strategy.

## Steps to generate bundles from an existing release.yaml

**Note:** The input release.yaml could be a github release or the result of `ko resolve config`

1. Make sure that we have the release manifest file ready
   eg:
   ```bash
   curl -L -o tektoncd-operator-0.49.0 https://github.com/tektoncd/operator/releases/download/v0.49.0/release.notags.yaml
   ```

2. From the project root (tektoncd/operator) run

    ```bash
    export BUNDLE_ARGS='--workspace operatorhub/kubernetes --operator-release-version 0.49.0 --channels alpha --default-channel alpha --fetch-strategy-release-manifest --release-manifest tektoncd-operator-0.49.0 --upgrade-strategy-semver'
    make operator-bundle
    ```

    **CLI flags explained**

    Flag                                          | Description
    --------------------------------------------- | -----------
    `--workspace operatorhub/kubernetes`          | the working directory where the operator bundle should be assembled
    `--operator-release-version 0.49.0`           | version of the release (version of bundle)
    `--channels alpha`                            | target release channel(s) (eg: stable,preview)
    `--default-channel alpha`                     | set default channel of the operator
    `--fetch-strategy-release-manifest`           | gather input kubernetes resources from a list of yaml manifests instead of using local files
    `--release-manifest tektoncd-operaotr-0.49.0` | specify release manifest file
    `--upgrade-strategy-semver`                   | specify update strategy (use `replaces` or `semver`)
    ````
