# How to generate Operator Bundles for Kubernetes

For Kubernetes Platform ([operatorhub.io](https://operatorhub.io)) release we generate the
bundle for using `release-manifest` strategy.

## Steps to generate bundles from an existing github release.yaml

**Note:** The input release.yaml could be a github release or the result of `ko resolve config`

1. Make sure that we have the release manifest file ready
   eg:
   ```bash
   tektoncd_operator_version=$( curl -sL https://api.github.com/repos/tektoncd/operator/releases | jq  -r '.[].tag_name' | sort -Vr | head -n 1 | tr -d 'v')
   release_file_name=release-v${tektoncd_operator_version}.yaml
   curl -sL -o ${release_file_name} https://github.com/tektoncd/operator/releases/download/v${tektoncd_operator_version}/release.notags.yaml
   ```

2. From the project root (tektoncd/operator) run (note abosolute path for `--release-manifest` flag is necessary)

    ```bash
    export BUNDLE_ARGS="--workspace kubernetes --operator-release-version ${tektoncd_operator_version} --channels alpha --default-channel alpha --fetch-strategy-release-manifest --release-manifest $(pwd)/${release_file_name} --upgrade-strategy-semver"
    make operator-bundle
    ```

   **CLI flags explained**

   | Flag                                                      | Description                                                                                  |
   | --------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
   | `--workspace kubernetes`                                  | the working directory (inside operatorhub/) where the operator bundle should be assembled    |
   | `--operator-release-version ${tektoncd_operator_version}` | version of the release (version of bundle)                                                   |
   | `--channels alpha`                                        | target release channel(s) (eg: stable,preview)                                               |
   | `--default-channel alpha`                                 | set default channel of the operator                                                          |
   | `--fetch-strategy-release-manifest`                       | gather input kubernetes resources from a list of yaml manifests instead of using local files |
   | `--release-manifest $(pwd)/${release_file_name}`          | specify abosolute ($(pwd)/${release_file_name}) path to release manifest file                |
   | `--upgrade-strategy-semver`                               | specify update strategy (options `replaces` or `semver`)                                     |
