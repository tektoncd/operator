# Tekton Pipelines Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release
of Tekton Operator. To follow these steps you'll need a checkout of
the operator repo, a terminal window and a text editor.

1. [Setup a context to connect to the dogfooding cluster](#setup-dogfooding-context) if you haven't already.

1. `cd` to root of Operator git checkout.

1. Select the commit you would like to build the release from, most likely the
   most recent commit at https://github.com/tektoncd/operator/commits/main
   and note the commit's hash.

1. Define the version of the operator. At present, the community consensus is to version operator releases
   based on the Tekton Pipeline Version provided by the operator in the release.

   `operator version = <pipeline-version-semver>-operator-release-build-number`

   eg: `0.21.0-1`: implies that this release will deliber Tetkon Pipelines `0.21.0` and this is the '1st'
   operator reelase for this Pipeline version. If operator has to be re-released for a particular pipeline version,
   then increament the `operator-release-build-number` (eg: `0.21.0-2`)

1. Create environment variables for bash scripts in later steps.

    ```bash
    TEKTON_VERSION=# Example: 0.21.0-1
    TEKTON_RELEASE_GIT_SHA=# SHA of the release to be released
    TEKTON_IMAGE_REGISTRY=tekton-releases # only change if you want to publish to a different registry
    ```

1. Confirm commit SHA matches what you want to release.

    ```bash
    git show $TEKTON_RELEASE_GIT_SHA
    ```
1. Create a workspace template file:

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

1. Execute the release pipeline.

    ```bash
    tkn --context dogfooding pipeline start operator-release \
      --param=gitRevision="${TEKTON_RELEASE_GIT_SHA}" \
      --param=versionTag="${TEKTON_VERSION}" \
      --param=imageRegistryPath="${TEKTON_IMAGE_REGISTRY}" \
      --param=serviceAccountPath=release.json \
      --param=releaseBucket=gs://tekton-releases/operator \
      --workspace name=release-secret,secret=release-secret \
      --workspace name=workarea,volumeClaimTemplateFile=workspace-template.yaml
    ```

1. Watch logs of resulting PipelineRun.

1. Once the pipeline is complete, check its results:

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

   The `commit-sha` should match `$TEKTON_RELEASE_GIT_SHA`.
   The two URLs can be opened in the browser or via `curl` to download the release manifests.

1. The YAMLs are now uploaded to publically accesible gcs bucket! Anyone installing Tekton Pipelines will now get the new version. Time to create a new GitHub release announcement:

    1. The release announcement draft is created by the [create-draf-release](https://github.com/tektoncd/plumbing/blob/main/tekton/resources/release/base/github_release.yaml) task.
       The task requires a `pipelineResource` to work with the operator repository. Create the pipelineresource:
       ```shell script
        cat <<EOF | kubectl apply -f -                                                                                                                                             130 ↵
        apiVersion: tekton.dev/v1alpha1
        kind: PipelineResource
        metadata:
          name: tekton-operator-git-v0-21-0-1
        spec:
          type: git
          params:
          - name: url
            value: https://github.com/tektoncd/operator
          - name: revision
            value: <commit SHA of the release to be released> #eg:01ac5500e0335c9cdadbe1a76e133bb33c13d87
        EOF

       ```
    1. Create additional environment variables

        ```bash
       VERSION_TAG=v0.21.0-1
       PREVIOUS_VERSION_TAG=v0.19.0-1
       GIT_RESOURCE_NAME=tekton-operator-git-v0-21-0-1
       IMAGE_REGISTRY=tekton-releases
        ```

    1. Execute the Draft Release task.

        ```bash
        tkn --context dogfooding task start \
          -i source="${GIT_RESOURCE_NAME}" \
          -i release-bucket=tekton-operator-bucket \
          -p package=tektoncd/operator \
          -p release-tag="${VERSION_TAG}" \
          -p previous-release-tag="${PREVIOUS_VERSION_TAG}" \
          -p release-name="" \
          create-draft-release
        ```

    1. Watch logs of create-draft-release

    1. On successful completion, a 👉 URL will be logged. Visit that URL and look through the release notes.
      1. Manually add upgrade and deprecation notices based on the generated release notes
      1. Double-check that the list of commits here matches your expectations
         for the release. You might need to remove incorrect commits or copy/paste commits
         from the release branch. Refer to previous releases to confirm the expected format.

    1. Un-check the "This is a pre-release" checkbox since you're making a legit for-reals release!

    1. Publish the GitHub release once all notes are correct and in order.

1. Edit `README.md` on `master` branch, add entry to docs table with latest release links.

1. Push & make PR for updated `README.md`

1. Test release that you just made against your own cluster (note `--context my-dev-cluster`):

    ```bash
    # Test latest
    kubectl --context my-dev-cluster apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
    ```

1. Announce the release in Slack channels #general and #pipelines.

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
