# How to generate Operator Bundles for Kubernetes

For Kubernetes Platform ([operatorhub.io](https://operatorhub.io)) release we generate the
bundle using the `release-manifest` strategy.

## Prerequisites

- **Container registry:** You can use either Quay.io or [ttl.sh](https://ttl.sh) (anonymous, no login).

  - **Quay.io:** Log in first (`docker login quay.io`). Set `IMAGE_NAMESPACE` to a Quay org/namespace you can push to (e.g. `your-username/tektoncd/operator`). The default `jkhelil/tektoncd/operator` is only an example and will fail with "unauthorized" unless you have access.

  - **ttl.sh (instead of Quay):** No login. Use ttl.sh so the catalog image is at a public URL like `ttl.sh/<name>/operator-catalog:0.79.0` instead of `quay.io/.../operator-catalog:0.79.0`. Set:
    ```bash
    export IMAGE_HOST=ttl.sh
    export IMAGE_NAMESPACE=tektoncd-operator   # or any single path segment, e.g. your-name-operator
    ```
  You can pass `IMAGE_HOST` and `IMAGE_NAMESPACE` on the command line (e.g. `make operator-bundle-push IMAGE_NAMESPACE=your-username/tektoncd/operator`) or edit `operatorhub/Makefile`.

- **Kubernetes cluster (for testing with OLM):** Steps 5 and 6 (install OLM and run the operator catalog) require a cluster. You do **not** need Kind specifically—any cluster is fine (Kind, Minikube, k3d, or a cloud cluster). Ensure `kubectl` is configured and your current context points to that cluster:
  ```bash
  kubectl config current-context
  kubectl cluster-info
  ```

## Steps to generate bundles from an existing github release.yaml

### Generate the operator bundle

**Note:** The input release.yaml could be a github release or the result of `ko resolve config`

1. **Set minKubeVersion (required by OperatorHub CI).** In **`operatorhub/kubernetes/config.yaml`** set `min-kube-version` to the minimum Kubernetes version your operator supports (e.g. `"1.28.0"`). When you run `make operator-bundle`, the bundle tooling (`bundle.py`) will add `spec.minKubeVersion` to the generated CSV automatically. Without this, the community-operators CI may fail with "csv.Spec.minKubeVersion is not informed".

2. Make sure that we have the release manifest file ready
   eg:
   ```bash
   tektoncd_operator_version=$( curl -sL https://api.github.com/repos/tektoncd/operator/releases | jq  -r '.[].tag_name' | sort -Vr | head -n 1 | tr -d 'v')
   release_file_name=release-v${tektoncd_operator_version}.yaml
   curl -sL -o ${release_file_name} "https://github.com/tektoncd/operator/releases/download/v${tektoncd_operator_version}/release.notags.yaml"
   ```

3. From the project root (tektoncd/operator) run (note absolute path for `--release-manifest` flag is necessary)

    ```bash
    export BUNDLE_ARGS="--workspace kubernetes --operator-release-version ${tektoncd_operator_version} --channels stable --default-channel stable --fetch-strategy-release-manifest --release-manifest $(pwd)/${release_file_name} --upgrade-strategy-semver"
    make operator-bundle
    ```

   **CLI flags explained**

   | Flag                                                      | Description                                                                                  |
   | --------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
   | `--workspace kubernetes`                                  | the working directory (inside operatorhub/) where the operator bundle should be assembled    |
   | `--operator-release-version ${tektoncd_operator_version}` | version of the release (version of bundle)                                                   |
   | `--channels stable`                                        | target release channel(s) (eg: stable,preview)                                               |
   | `--default-channel stable`                                | set default channel of the operator                                                          |
   | `--fetch-strategy-release-manifest`                       | gather input kubernetes resources from a list of yaml manifests instead of using local files |
   | `--release-manifest $(pwd)/${release_file_name}`          | specify absolute ($(pwd)/${release_file_name}) path to release manifest file                 |
   | `--upgrade-strategy-semver`                               | specify update strategy (options `replaces` or `semver`)                                     |

Note: the generated files are the base bundle files that serve to create a PR in https://github.com/k8s-operatorhub/community-operators/

### Test the operator bundle

**Version:** Set `VERSION` in `operatorhub/Makefile` (default `0.79.0`) to match the version you use when generating the bundle.

1. Build the operator bundle image using

   ```bash
   make operator-bundle-build
   ```

2. Push the operator bundle image to a container registry.

   - Log in first (e.g. for Quay: `docker login quay.io`).
   - Set `IMAGE_HOST` and `IMAGE_NAMESPACE` to a registry and namespace you can push to (e.g. `make operator-bundle-push IMAGE_NAMESPACE=your-username/tektoncd/operator` or edit operatorhub/Makefile). The default `jkhelil/tektoncd/operator` is only an example.

   ```bash
   make operator-bundle-push
   ```

3. Build the operator catalog source image

   ```bash
   make operator-catalog-build
   ```

4. Push the operator catalog source image

   ```bash
   make operator-catalog-push
   ```

5. Install OLM (requires a Kubernetes cluster; see [Prerequisites](#prerequisites))

   ```bash
   make install-olm
   ```

6. Run the operator catalog

   ```bash
   make operator-catalog-run
   ```

7. Verify the installation: TektonConfig should be up and ready at the end of testing. Check that the operator has created and reconciled the config:

   ```bash
   kubectl get tektonconfig -A
   ```

   You should see a `config` TektonConfig with `READY` True and your bundle version (e.g. `v0.79.0`). If it is not ready, check operator and catalog pods and logs.

