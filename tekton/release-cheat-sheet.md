# Tekton Operator Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release of Tekton Operator. To follow these steps you'll
need a checkout of the operator repo, a terminal window and a text editor.

## Pre-requisites

1. tektoncd/operator repository is cloned locally and a remote with name `tektoncd` points to
   `github.com/tektoncd/operator` repository.

## Setting up Release Branch

1. Select the commit you would like to build the release from, most likely the most recent commit
   at https://github.com/tektoncd/operator/commits/main
   and note the commit's hash.

2. Define the version of the operator. At present, for each release we increment the minor version of operator. For more
   details, refer [Tektoncd Operator Release](../docs/release/README.md). eg: `0.62.0` or `0.62.1` (patch release)

   Set the version in a variable.

    ```bash
    TEKTON_RELEASE_VERSION=v0.62.0
   ```

3. Set the release branch name

   **minor version release vs patch release:**
    1. If this is a **new minor version release** create a new branch from either the head of `#main` branch if the
       commit identified in step1.

       ```bash
   TEKTON_RELEASE_BRANCH=release-v0.62.x git checkout -b ${TEKTON_RELEASE_BRANCH}
   ```
    2. If this is a **patch release** make sure that the correct branch is checkout. eg: If we are making release
       v0.62.1, then make sure the `release-v0.62.x` is checked out.

5. Make sure the Tektoncd Component versions are the one you want, in
   `components.yaml`. Those are kept up-to-date by our bots, but just
   in case.

6. minor version release vs patch release:
    1. if this is a minor version release push the branch to `github.com/tektoncd/operator`

    ```bash
    git push tektoncd ${TEKTON_RELEASE_BRANCH}
    ```

    2. if this is a patch release, make a pull request to the appropriate minor version release branch (eg:
       release-v0.62.x)
       and get it merged before continuing to the next section.

## Running Release Pipeline

1. [Setup a context to connect to the dogfooding cluster](#setup-dogfooding-context) if you haven't already.

2`cd` to root of Operator git checkout.

3. Make sure the release `Task` and `Pipeline` are up-to-date on the
   cluster. To do that, you can use `kustomize`:
   
   ```bash
   kustomize build tekton | kubectl replace -f -
   ```

    - [publish-operator-release](https://github.com/tektoncd/operator/blob/main/tekton/build-publish-images-manifests.yaml)

      This task uses [ko](https://github.com/google/ko) to build all container images we release and generate
      the `release.yaml`
      ```shell script
      kubectl apply -f tekton/bases/build-publish-images-manifests.yaml
      ```
    - [operator-release](https://github.com/tektoncd/operator/blob/main/tekton/operator-release-pipeline.yaml)
      ```shell script
      kubectl apply -f tekton/overlays/versioned-releases/operator-release-pipeline.yaml
      ```

4. Confirm commit SHA matches what you want to release.

    ```bash
    git show $TEKTON_RELEASE_GIT_SHA
    ```

6. Create a workspace template file:

   ```bash
   cat <<EOF > workspace-template.yaml
   spec:
     accessModes:
     - ReadWriteOnce
     resources:
       requests:
         storage: 1Gi
   EOF
   ```

7. Execute the release pipeline.

    ```bash
    tkn --context dogfooding pipeline start operator-release \
        --serviceaccount=release-right-meow \
        --param=components=components.yaml \
        --param=gitRevision="${TEKTON_RELEASE_GIT_SHA}" \
        --param=versionTag="${TEKTON_RELEASE_VERSION}" \
        --param=serviceAccountPath=release.json \
        --param=releaseBucket=gs://tekton-releases/operator \
        --param=imageRegistry=gcr.io \
        --param=imageRegistryPath=tekton-releases  \
        --param=releaseAsLatest=true \
        --param=platforms=linux/amd64,linux/arm64,linux/s390x,linux/ppc64le \
        --param=kubeDistros="kubernetes openshift" \
        --param=package=github.com/tektoncd/operator \
        --workspace name=release-secret,secret=release-secret \
        --workspace name=workarea,volumeClaimTemplateFile=workspace-template.yaml \
        --pipeline-timeout 2h0m0s
    ```

8. Watch logs of resulting PipelineRun.

9. Once the pipeline is complete, check its results:

   ```bash
   tkn pr describe <pipeline-run-name>

   (...)
   📝 Results

   NAME                    VALUE
   ∙ commit-sha            ff6d7abebde12460aecd061ab0f6fd21053ba8a7
   ∙ release-file           https://storage.googleapis.com/tekton-releases/operator/previous/v20210223-xyzxyz/release.yaml
   ∙ release-file-no-tag    https://storage.googleapis.com/tekton-releases/operator/previous/v20210223-xyzxyz/release.notag.yaml

   (...)
   ```

   The `commit-sha` should match `$TEKTON_RELEASE_GIT_SHA`. The two URLs can be opened in the browser or via `curl` to
   download the release manifests.

## Creating Github Release

1. The YAMLs are now uploaded to publically accesible gcs bucket! Anyone installing Tekton Pipelines will now get the
   new version. Time to create a new GitHub release announcement:

    1. Create additional environment variables

    ```bash
    TEKTON_OLD_VERSION=# Example: v0.11.1
    TEKTON_RELEASE_NAME=# The release name you just chose, e.g.: "Ragdoll Norby"
    ```

    1. Execute the Draft Release Pipeline.

    ```bash
    tkn --context dogfooding pipeline start \
      --workspace name=shared,volumeClaimTemplateFile=workspace-template.yaml \
      --workspace name=credentials,secret=release-secret \
      -p package="tektoncd/operator" \
      -p git-revision="$TEKTON_RELEASE_GIT_SHA" \
      -p release-tag="${TEKTON_RELEASE_VERSION}" \
      -p previous-release-tag="${TEKTON_OLD_VERSION}" \
      -p release-name="" \
      -p bucket="gs://tekton-releases/operator" \
      -p rekor-uuid="" \
      release-draft
    ```

    1. Watch logs of create-draft-release

    1. On successful completion, a 👉 URL will be logged. Visit that URL and look through the release notes. 1. Manually
       add upgrade and deprecation notices based on the generated release notes 1. Double-check that the list of commits
       here matches your expectations for the release. You might need to remove incorrect commits or copy/paste commits
       from the release branch. Refer to previous releases to confirm the expected format.

    1. Un-check the "This is a pre-release" checkbox since you're making a legit for-reals release!

    1. Publish the GitHub release once all notes are correct and in order.

2. Edit `README.md` on `master` branch, add entry to docs table with latest release links.

3. Push & make PR for updated `README.md`

4. Test release that you just made against your own cluster (note `--context my-dev-cluster`):

    ```bash
    # Test latest
    kubectl --context my-dev-cluster apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
    ```

5. Announce the release in Slack channels #general and #pipelines.

Congratulations, you're done!

## Setup dogfooding context

1. Configure `kubectl` to connect to
   [the dogfooding cluster](https://github.com/tektoncd/plumbing/blob/master/docs/dogfooding.md):

    ```bash
    gcloud container clusters get-credentials dogfooding --zone us-central1-a --project tekton-releases
    ```

1. Give [the context](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
   a short memorable name such as `dogfooding`:

   ```bash
   kubectl config rename-context gke_tekton-releases_us-central1-a_dogfooding dogfooding
   ```

1. **Important: Switch `kubectl` back to your own cluster by default.**

    ```bash
    kubectl config use-context my-dev-cluster
    ```
