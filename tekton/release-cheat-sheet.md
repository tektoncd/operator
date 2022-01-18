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
   details, refer [Tektoncd Operator Release](../docs/release/README.md). eg: `0.52.0` or `0.52.1` (patch release)

   Set the version in a variable.

    ```bash
    TEKTON_RELEASE_VERSION=v0.52.0
   ```

3. Set the release branch name

   **minor version release vs patch release:**
    1. If this is a **new minor version release** create a new branch from either the head of `#main` branch if the
       commit identified in step1.

       ```bash
   TEKTON_RELEASE_BRANCH=release-v0.52.x git checkout -b ${TEKTON_RELEASE_BRANCH}
   ```
    2. If this is a **patch release** make sure that the correct branch is checkout. eg: If we are making release
       v0.52.1, then make sure the `release-v0.52.x` is checked out.

5. Update the Tektoncd Component versions in `test/config.sh` in the project root. This is necessary to pin the
   component versions in e2e tests for a versioned branch

   eg:
     ```bash
     export TEKTON_PIPELINE_VERSION=v0.30.0
     export TEKTON_TRIGGERS_VERSION=v0.17.1
     export TEKTON_RESULTS_VERSION=v0.3.1
     export TEKTON_DASHBOARD_VERSION=v0.22.0
     ```

   commit the `test/config.sh` file.

6. minor version release vs patch release:
    1. if this is a minor version release push the branch to `github.com/tektoncd/operator`

    ```bash
    git push tektoncd ${TEKTON_RELEASE_BRANCH}
    ```

    2. if this is a patch release, make a pull request to the appropriate minor version release branch (eg:
       release-v0.52.x)
       and get it merged before continuing to the next section.

## Running Release Pipeline

1. [Setup a context to connect to the dogfooding cluster](#setup-dogfooding-context) if you haven't already.

2`cd` to root of Operator git checkout.

3. Make sure the release `Task` and `Pipeline` are up-to-date on the cluster.

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

4. Create environment variables for bash scripts in later steps.

    ```bash
    TEKTON_RELEASE_VERSION=# Example: v0.52.0
    TEKTON_RELEASE_GIT_SHA=# SHA of the release to be released
    TEKTON_PIPELINE_VERSION=# v0.28.0
    TEKTON_TRIGGERS_VERSION=# v0.27.0
    TEKTON_DASHBOARD_VERSION=# v0.21.0
    TEKTON_RESULTS_VERSION=# v0.1.1
    ```

5. Confirm commit SHA matches what you want to release.

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
      --param=gitRevision="${TEKTON_RELEASE_GIT_SHA}" \
      --param=versionTag="${TEKTON_RELEASE_VERSION}" \
      --param=TektonCDPipelinesVersion=${TEKTON_PIPELINE_VERSION} \
      --param=TektonCDTriggersVersion=${TEKTON_TRIGGERS_VERSION} \
      --param=TektonCDDashboardVersion=${TEKTON_DASHBOARD_VERSION} \
      --param=TektonCDResultsVersion=${TEKTON_RESULTS_VERSION} \
      --param=serviceAccountPath=release.json \
      --param=releaseBucket=gs://tekton-releases/operator \
      --workspace name=release-secret,secret=release-secret \
      --workspace name=workarea,volumeClaimTemplateFile=workspace-template.yaml
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
        TEKTON_PREV_RELEASE_VERSION=# Example: v0.11.1
        TEKTON_PACKAGE=tektoncd/operator
        ```

    1. The release announcement draft is created by
       the [create-draft-release](https://github.com/tektoncd/plumbing/blob/main/tekton/resources/release/base/github_release.yaml)
       task. The task requires a `pipelineResource` to work with the operator repository. Create the pipelineresource:
       ```shell script
       cat <<EOF | kubectl --context dogfooding create -f -
       apiVersion: tekton.dev/v1alpha1
       kind: PipelineResource
       metadata:
         name: tekton-operator-$(echo $TEKTON_RELEASE_VERSION | tr '.' '-')
         namespace: default
       spec:
         type: git
         params:
           - name: url
             value: 'https://github.com/tektoncd/operator'
           - name: revision
             value: ${TEKTON_RELEASE_GIT_SHA}
       EOF
       ```
       cat <<EOF | kubectl --context dogfooding create -f -

    1. Execute the Draft Release task.

        ```bash
        tkn --context dogfooding task start \
          -i source="tekton-operator-$(echo $TEKTON_RELEASE_VERSION | tr '.' '-')" \
          -i release-bucket=tekton-operator-bucket \
          -p package="${TEKTON_PACKAGE}" \
          -p release-tag="${TEKTON_RELEASE_VERSION}" \
          -p previous-release-tag="${TEKTON_PREV_RELEASE_VERSION}" \
          -p release-name="" \
          create-draft-release
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
