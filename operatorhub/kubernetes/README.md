# How to generate Operator Bundles for Kubernetes

For Kubernetes Platform ([operatorhub.io](https://operatorhub.io)) release we generate the
bundle for using `release-manifest` strategy.

## Steps to generate bundles from an existing github release.yaml

### Generate the operator bundle

**Note:** The input release.yaml could be a github release or the result of `ko resolve config`

1. Make sure that we have the release manifest file ready
   eg:
   ```bash
   tektoncd_operator_version=$( curl -sL https://api.github.com/repos/tektoncd/operator/releases | jq  -r '.[].tag_name' | sort -Vr | head -n 1 | tr -d 'v')
   release_file_name=release-v${tektoncd_operator_version}.yaml
   curl -sL -o ${release_file_name} "https://github.com/tektoncd/operator/releases/download/v${tektoncd_operator_version}/release.notags.yaml"
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

Note: the generated files are the base bundle files that severs to createa PR in https://github.com/k8s-operatorhub/community-operators/


### Test the operator bundle

1. Build the operator bundle image using

```bash
   make operator-bundle-build 
```

2. Push the operator bundle image to a container registry

```bash
   make operator-bundle-build 
```

2. Push the operator bundle image to a container registry
change IMAGE_HOST and IMAGE_NAMESPACE in operatorhub/Makefile, then
```bash
   make operator-bundle-build 
```

3. Build the operator catalog source image
```bash
   make operator-catalog-build 
```

4. Push the operator catalog source image
```bash
   make operator-catalog-push 
```

6. Install olm
```bash
   make install-olm
```

5. Run the operator catalog
```bash
   make operator-catalog-run
```
