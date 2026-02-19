# Tekton Operator Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release of Tekton Operator. To follow these steps you'll
need a checkout of the operator repo, a terminal window and a text editor.

## Pre-requisites

1. tektoncd/operator repository is cloned locally and a remote with name `tektoncd` points to
   `github.com/tektoncd/operator` repository.

## Setting up Release Branch

1. Select the commit you would like to build the release from, most likely the most recent commit
   at [https://github.com/tektoncd/operator/commits/main](https://github.com/tektoncd/operator/commits/main) 
   and note the commit's hash.

2. For each release, we increment the minor version of the operator. For more details, 
   refer to [Tektoncd Operator Release](../docs/release/README.md). 
   Example versions: `0.62.0` (minor release) or `0.62.1` (patch release).

   Set the version and old version in a variable.

   ```bash
    TEKTON_RELEASE_VERSION=v0.62.0
   ```


3. Make sure the Tektoncd Component versions are the one you want, in
   `components.yaml`. Those are kept up-to-date by our bots, but just
   in case.


4. Set the release branch and update yaml files
   
   1. **For Minor Version Release:** Use the script to create and set up the release branch. Example
      ```bash
         TEKTON_RELEASE_BRANCH=release-v0.62.x
         # ./hack/release-setup-branch.sh <old release version> <new release version>
         ./hack/release-setup-branch.sh devel ${TEKTON_RELEASE_VERSION#v}
      ```
      The script will automatically create a new branch for the minor version (e.g., release-v0.62.x) and switch to it.
      It updates the yaml files with the TEKTON_RELEASE_VERSION
   
   1. **For Patch Version Release:** Use the script to set up the release branch. Example
      ```bash
         TEKTON_RELEASE_BRANCH=release-v0.62.x
         # ./hack/release-setup-branch.sh <old release patch version> <new release patch version>
         ./hack/release-setup-branch.sh 0.62.0 0.62.1
      ```
      The script will automatically switch to the branch version. It updates the yaml files with the new release patch version.
      It convey the user to create a PR with patch changes to the release branch

5. **For Patch Version Release:** Ensure that the pull request created earlier has been merged before proceeding.

6. Update Helm charts with the new version. If applicable, update CRDs in the charts with new types.  
   ```bash
   # Update labels in YAML files under templates directory
   find charts/tekton-operator/templates -type f -name '*.yaml' -exec sed -i "s/operator\.tekton\.dev\/release: \"devel\"/operator.tekton.dev\/release: ${TEKTON_RELEASE_VERSION}/g" {} +
   find charts/tekton-operator/templates -type f -name '*.yaml' -exec sed -i "s/version: \"devel\"/version: ${TEKTON_RELEASE_VERSION}/g" {} +

   # Update Chart.yaml
   sed -i "s/^version: \"devel\"/version: ${TEKTON_RELEASE_VERSION#v}/" charts/tekton-operator/Chart.yaml
   sed -i "s/^appVersion: \"devel\"/appVersion: ${TEKTON_RELEASE_VERSION}/" charts/tekton-operator/Chart.yaml
   ```

7. Update Helm chart with latest changes and create a pull request for merging into the new branch.

## Running Release Pipeline

1. [Setup a context to connect to the dogfooding cluster](#setup-dogfooding-context) if you haven't already.

2. `cd` to root of Operator git checkout.

3. Create a `release.env` file with environment variables for bash scripts in later steps, and source it:

    ```bash
    cat <<EOF > release.env
    TEKTON_RELEASE_VERSION= # Example: v0.69.0
    TEKTON_OLD_VERSION= # Example: v0.68.0
    TEKTON_RELEASE_NAME="Oriental Longhair Omnibot" # Name of the release (should be a bird name) 
    TEKTON_PACKAGE=tektoncd/operator
    TEKTON_REPO_NAME=operator
    EOF
    . ./release.env
    ```


4. Set commit SHA from TEKTON_RELEASE_BRANCH:
   ```bash
   TEKTON_RELEASE_GIT_SHA=$(git rev-parse upstream/${TEKTON_RELEASE_BRANCH})
   ```

5. Confirm commit SHA matches what you want to release.

    ```bash
    git show $TEKTON_RELEASE_GIT_SHA
    ```

6. Create a workspace template file:

   ```bash
   WORKSPACE_TEMPLATE=$(mktemp /tmp/workspace-template.XXXXXX.yaml)
   cat <<'EOF' > $WORKSPACE_TEMPLATE
   spec:
     accessModes:
     - ReadWriteOnce
     resources:
       requests:
         storage: 1Gi
   EOF
   ```

7. Execute the release pipeline (takes ~45 mins).
    
    **The minimum required tkn version is v0.30.0 or later**

    **If you are back-porting include this flag: `--param=releaseAsLatest="false"`**
    ```bash
    tkn --context dogfooding pipeline start operator-release \
        --filename=tekton/operator-release-pipeline.yaml \
        --param package=github.com/tektoncd/operator \
        --param repoName="${TEKTON_REPO_NAME}" \
        --param components=components.yaml \
        --param gitRevision="${TEKTON_RELEASE_GIT_SHA}" \
        --param imageRegistry=ghcr.io \
        --param imageRegistryPath=tektoncd/operator  \
        --param imageRegistryRegions="" \
        --param imageRegistryUser=tekton-robot \
        --param serviceAccountImagesPath=credentials \
        --param versionTag="${TEKTON_RELEASE_VERSION}" \
        --param releaseBucket=tekton-releases \
        --param koExtraArgs="" \
        --param releaseAsLatest=true \
        --param platforms=linux/amd64,linux/arm64,linux/s390x,linux/ppc64le \
        --param kubeDistros="kubernetes openshift" \
        --workspace name=release-secret,secret=oci-release-secret \
        --workspace name=release-images-secret,secret=ghcr-creds \
        --workspace name=workarea,volumeClaimTemplateFile="${WORKSPACE_TEMPLATE}" \
        --pipeline-timeout 2h0m0s
    ```

7. Watch logs of resulting PipelineRun.

8. Once the pipeline is complete, check its results:

   ```bash
   tkn pr describe <pipeline-run-name>

   (...)
   📝 Results

   NAME                    VALUE
   ∙ commit-sha            ff6d7abebde12460aecd061ab0f6fd21053ba8a7
   ∙ release-file           https://infra.tekton.dev/tekton-releases/operator/previous/v20210223-xyzxyz/release.yaml
   ∙ release-file-no-tag    https://infra.tekton.dev/tekton-releases/operator/previous/v20210223-xyzxyz/release.notags.yaml

   (...)
   ```

   The `commit-sha` should match `$TEKTON_RELEASE_GIT_SHA`. The two URLs can be opened in the browser or via `curl` to
   download the release manifests.

## Creating Github Release

1. The YAMLs are now released. Anyone installing Tekton Operator will now get the new version. Time to create a new GitHub release announcement:

    1. Find the Rekor UUID for the release:

        ```bash
        RELEASE_FILE=https://infra.tekton.dev/tekton-releases/operator/previous/${TEKTON_RELEASE_VERSION}/release.yaml
        OPERATOR_IMAGE_SHA=$(curl -L $RELEASE_FILE | sed -n 's/"//g;s/.*ghcr\.io\/tektoncd\/operator\/operator-[^@]*@//p;' | head -1)
        REKOR_UUID=$(rekor-cli search --sha $OPERATOR_IMAGE_SHA | grep -v Found | head -1)
        echo -e "OPERATOR_IMAGE_SHA: ${OPERATOR_IMAGE_SHA}\nREKOR_UUID: ${REKOR_UUID}"
        ```

    2. Execute the Draft Release Pipeline.

        Create a pod template file:

        ```shell
        POD_TEMPLATE=$(mktemp /tmp/pod-template.XXXXXX.yaml)
        cat <<'EOF' > $POD_TEMPLATE
        securityContext:
          fsGroup: 65532
          runAsUser: 65532
          runAsNonRoot: true
        EOF
        ```

        ```bash
        tkn --context dogfooding pipeline start \
          --workspace name=shared,volumeClaimTemplateFile="${WORKSPACE_TEMPLATE}" \
          --workspace name=credentials,secret=oci-release-secret \
          --pod-template "${POD_TEMPLATE}" \
          -p package="${TEKTON_PACKAGE}" \
          -p git-revision="${TEKTON_RELEASE_GIT_SHA}" \
          -p release-tag="${TEKTON_RELEASE_VERSION}" \
          -p previous-release-tag="${TEKTON_OLD_VERSION}" \
          -p release-name="${TEKTON_RELEASE_NAME}" \
          -p repo-name="${TEKTON_REPO_NAME}" \
          -p bucket="tekton-releases" \
          -p rekor-uuid="${REKOR_UUID}" \
          release-draft-oci
        ```

    3. Watch logs of create-draft-release

    4. On successful completion, a 👉 URL will be logged. Visit that URL and look through the release notes.

    5. Manually add upgrade and deprecation notices based on the generated release notes.

    6. Double-check that the list of commits here matches your expectations for the release. You might need to remove incorrect commits or copy/paste commits from the release branch. Refer to previous releases to confirm the expected format.

    7. Un-check the "This is a pre-release" checkbox since you're making a legit for-reals release!

    8. Publish the GitHub release once all notes are correct and in order.

2. Edit `README.md` on `master` branch, add entry to docs table with latest release links.
   In README.md, update the supported versions and end-of-life sections:

   Add ${TEKTON_RELEASE_VERSION} under the Supported Versions section (### In Support).
   Move the previous version to the End of Life section (### End of Life).


3. Push & make PR for updated `README.md`

4. Test release that you just made against your own cluster (note `--context my-dev-cluster`):

    ```bash
    # Test latest
    kubectl --context my-dev-cluster apply --filename https://infra.tekton.dev/tekton-releases/pipeline/latest/release.yaml
    ```

5. Announce the release in Slack channels #general and #pipelines.

Congratulations, you're done!
   

## Setup dogfooding context

1. Configure `kubectl` to connect to
   [the dogfooding cluster](https://github.com/tektoncd/plumbing/blob/main/docs/dogfooding.md):

    ```bash
    oci ce cluster create-kubeconfig --cluster-id <CLUSTER-OCID> --file $HOME/.kube/config --region <CLUSTER-REGION> --token-version 2.0.0  --kube-endpoint PUBLIC_ENDPOINT
    ```

2. Give [the context](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
   a short memorable name such as `dogfooding`:

   ```bash
   kubectl config rename-context <REPLACE-WITH-NAME-FROM-CONFIG-CONTEXT> dogfooding
   ```

3. **Important: Switch `kubectl` back to your own cluster by default.**

    ```bash
    kubectl config use-context my-dev-cluster
    ```

## Ensure documentation is updated

In the https://github.com/tektoncd/website.git repository, under sync/config/operator.yaml,
ensure that you add a section for the new ${TEKTON_RELEASE_VERSION} version.

## Add operator to operatorhub

This process is typically done for minor releases, but not for patch releases.

1. Under the https://github.com/k8s-operatorhub/community-operators.git repository, follow these instructions:
2. Walk through the file /operatorhub/kubernetes/README.md to generate the bundle.
3. Copy the bundle generated to your local community-operators repository.
4. After completing the steps to generate and copy the bundle, create a pull request against the community-operators repository
