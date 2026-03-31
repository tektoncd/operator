# Tekton Operator Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release
of Tekton Operator.

## Automated Releases (Pipelines-as-Code)

Releases are automated using [Pipelines-as-Code](https://pipelinesascode.com).
The release PipelineRuns are defined in the `.tekton/` directory and executed
on the shared Tekton infrastructure cluster.

### Initial Release (minor)

1. Select the commit you would like to build the release from on the `main` branch.

1. Make sure the Tektoncd Component versions are the one you want in
   `components.yaml`. Those are kept up-to-date by our bots, but just in case.

1. Use the release setup script to prepare the release branch:

   ```bash
   TEKTON_RELEASE_VERSION=v0.80.0
   TEKTON_RELEASE_BRANCH=release-v0.80.x
   # ./hack/release-setup-branch.sh <old release version> <new release version>
   ./hack/release-setup-branch.sh devel ${TEKTON_RELEASE_VERSION#v}
   ```

   The script creates a new branch `release-v0.80.x`, updates version labels,
   and commits the changes.

1. Push the release branch:

   ```bash
   git push upstream release-v0.80.x
   ```

   Pipelines-as-Code automatically detects the branch creation and triggers the
   release pipeline (`.tekton/release.yaml`). The version is derived from the
   branch name (`release-v0.80.x` → `v0.80.0`).

1. Monitor the PipelineRun on the [Tekton Dashboard](https://tekton.infra.tekton.dev/#/namespaces/releases-operator/pipelineruns)
   or via `tkn pac logs -n releases-operator -L`.

1. On successful completion, check the release artifacts:

   ```bash
   # Kubernetes release manifest
   curl -s https://infra.tekton.dev/tekton-releases/operator/previous/${TEKTON_RELEASE_VERSION}/release.yaml | head

   # OpenShift release manifest
   curl -s https://infra.tekton.dev/tekton-releases/operator/previous/${TEKTON_RELEASE_VERSION}/openshift-release.yaml | head
   ```

1. Update Helm charts with the new version:

   ```bash
   # Update labels in YAML files under templates directory
   find charts/tekton-operator/templates -type f -name '*.yaml' -exec sed -i \
     "s/operator\.tekton\.dev\/release: \"devel\"/operator.tekton.dev\/release: ${TEKTON_RELEASE_VERSION}/g" {} +
   find charts/tekton-operator/templates -type f -name '*.yaml' -exec sed -i \
     "s/version: \"devel\"/version: ${TEKTON_RELEASE_VERSION}/g" {} +

   # Update Chart.yaml
   sed -i "s/^version: \"devel\"/version: ${TEKTON_RELEASE_VERSION#v}/" charts/tekton-operator/Chart.yaml
   sed -i "s/^appVersion: \"devel\"/appVersion: ${TEKTON_RELEASE_VERSION}/" charts/tekton-operator/Chart.yaml
   ```

### Patch Release (bugfix)

Patch releases can be triggered manually or automatically.

#### Manual trigger (workflow_dispatch)

1. Ensure that any cherry-picked PRs have been merged to the release branch.

1. Use the release setup script on the existing release branch:

   ```bash
   TEKTON_RELEASE_BRANCH=release-v0.79.x
   # ./hack/release-setup-branch.sh <old release patch version> <new release patch version>
   ./hack/release-setup-branch.sh 0.79.0 0.79.1
   ```

   The script updates version labels for the patch release. Create a PR with
   these changes and merge it before proceeding.

1. Go to [Actions → Patch Release](https://github.com/tektoncd/operator/actions/workflows/patch-release.yaml)
1. Click **Run workflow**
1. Fill in the release branch (e.g. `release-v0.79.x`) and version (e.g. `v0.79.1`)
1. Set "Publish as latest release" appropriately
1. Click **Run workflow**

The workflow triggers the release pipeline via PAC incoming webhook.

#### Automatic trigger (weekly cron)

A cron job runs every Thursday at 10:00 UTC. It scans all active release
branches (≥ v0.70) for commits since the last tag. If new commits are found,
it automatically triggers a patch release via the PAC incoming webhook.

## Creating Github Release

1. **If the `github-secret` workspace was provided** (default for PAC releases),
   a draft GitHub release is created automatically by the pipeline after Chains
   signing completes. Skip to step 3.

1. **If the `github-secret` workspace was NOT provided**, you can create the
   draft manually:

    Find the Rekor UUID for the release:

    ```bash
    RELEASE_FILE=https://infra.tekton.dev/tekton-releases/operator/previous/${TEKTON_RELEASE_VERSION}/release.yaml
    OPERATOR_IMAGE_SHA=$(curl -L $RELEASE_FILE | sed -n 's/"//g;s/.*ghcr\.io\/tektoncd\/operator\/operator-[^@]*@//p;' | head -1)
    REKOR_UUID=$(rekor-cli search --sha $OPERATOR_IMAGE_SHA | grep -v Found | head -1)
    echo -e "OPERATOR_IMAGE_SHA: ${OPERATOR_IMAGE_SHA}\nREKOR_UUID: ${REKOR_UUID}"
    ```

    Execute the Draft Release Pipeline:

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

    POD_TEMPLATE=$(mktemp /tmp/pod-template.XXXXXX.yaml)
    cat <<'EOF' > $POD_TEMPLATE
    securityContext:
      fsGroup: 65532
      runAsUser: 65532
      runAsNonRoot: true
    EOF

    tkn --context dogfooding pipeline start \
      --workspace name=shared,volumeClaimTemplateFile="${WORKSPACE_TEMPLATE}" \
      --workspace name=credentials,secret=github-token \
      --pod-template "${POD_TEMPLATE}" \
      -p package="tektoncd/operator" \
      -p git-revision="${TEKTON_RELEASE_GIT_SHA}" \
      -p release-tag="${TEKTON_RELEASE_VERSION}" \
      -p previous-release-tag="${TEKTON_OLD_VERSION}" \
      -p release-name="${TEKTON_RELEASE_NAME}" \
      -p repo-name="operator" \
      -p bucket="tekton-releases" \
      -p rekor-uuid="${REKOR_UUID}" \
      release-draft-oci
    ```

1. On successful completion, visit https://github.com/tektoncd/operator/releases
   and review the draft:
   - Manually add upgrade and deprecation notices
   - Double-check the list of commits matches expectations
   - Un-check "This is a pre-release"
   - Publish the release

## Post-Release Steps

1. Edit `README.md` on `main` branch. Update the supported versions and
   end-of-life sections:
   - Add the new version under "In Support"
   - Move the previous version to "End of Life"

1. Push & make PR for updated `README.md`.

1. Test release against your own cluster:

    ```bash
    # Test latest
    kubectl apply --filename https://infra.tekton.dev/tekton-releases/operator/latest/release.yaml

    # Test backport
    kubectl apply --filename https://infra.tekton.dev/tekton-releases/operator/previous/v0.79.1/release.yaml
    ```

1. Announce the release in Slack channels #general and #pipelines.

## Cherry-picking commits for patch releases

The easiest way to cherry-pick a commit into a release branch is to use the
"cherrypicker" plugin. Comment `/cherry-pick <branch>` on the pull request
containing the commits that need to be cherry-picked. Use one comment per
branch. Automation will create a pull request cherry-picking the commits.

If there are merge conflicts, manually cherry-pick:

```sh
git fetch upstream <branchname>
git checkout upstream/<branchname>
git cherry-pick <commit-hash>
# Resolve conflicts, then:
git add <changed-files>
git cherry-pick --continue
git push <your-fork> HEAD:<new-branch>
# Open PR against upstream/<branchname>
```

## Add operator to operatorhub

This process is typically done for minor releases, but not for patch releases.

1. Under the https://github.com/k8s-operatorhub/community-operators.git repository:
2. Walk through the file `/operatorhub/kubernetes/README.md` to generate the bundle.
3. Copy the bundle generated to your local community-operators repository.
4. Create a pull request against the community-operators repository.

## Infrastructure

- **PAC controller**: https://pac.infra.tekton.dev
- **Tekton Dashboard**: https://tekton.infra.tekton.dev
- **Release namespace**: `releases-operator`
- **Release bucket**: `tekton-releases` (Oracle Cloud Storage)
- **Image registry**: `ghcr.io/tektoncd/operator`
