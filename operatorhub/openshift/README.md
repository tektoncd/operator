# How to generate Operator Bundles for OpenShift

For OpenShift Platform release we generate the
bundle for using `local` strategy.

## Steps to generate bundles from an existing release.yaml

**Note:** The input release.yaml could be a github release or the result of `ko resolve config`

1. From the project root (tektoncd/operator) run

    ```bash
    OPERATOR_RELEASE_VERSION=x.y.z
    PREVIOUS_OPERATOR_RELEASE_VERSION=a.b.c
    export BUNDLE_ARGS="--workspace operatorhub/openshift --operator-release-version ${OPERATOR_RELEASE_VERSION} --channels stable,preview --default-channel stable --fetch-strategy-local --upgrade-strategy-replaces --operator-release-previous-version ${PREVIOUS_OPERATOR_RELEASE_VERSION}"
    make operator-bundle
    ```

   **CLI flags explained**

   Flag                                          | Description
       --------------------------------------------- | -----------
   `--workspace operatorhub/openshift`           | the working directory where the operator bundle should be assembled
   `--operator-release-version 1.6.0`            | version of the release (version of bundle)
   `--channels stable,preview`                   | target release channel(s) (eg: stable,preview)
   `--default-channel stable`                    | set default channel of the operator
   `--fetch-strategy-local`                      | gather input resources definitions from a local yaml files
   `--upgrade-strategy-replaces`                 | specify update strategy (use `replaces` or `semver`)
   `--operator-release-previous-version 1.5.0`   | version of the previous operator release that will be replaced by the bundle being built
    ````
