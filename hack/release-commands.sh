#!/usr/bin/env bash

SCRIPT_DIR=$(dirname $0)

source ${SCRIPT_DIR}/release-const.sh
source ${SCRIPT_DIR}/release-common.sh
source ${SCRIPT_DIR}/release-freeze-component-versions.sh

function print_release_commands() {
  local release_git_sha=${1}; shift
  local release_version=v${1}; shift
  local previous_release_version=v${1}

  divider
  printf "Tektocd Operator Release: ${release_version}\n\n"
  printf "Run the following commands to make a tektoncd/operator release\n"
  divider

  printf "1. Create a workspace template file (if it does not exist)\n"
  workspace_template_create_command
  divider

  printf "2. Start release pipeline run command:\n"
  print_pipeline_start_command ${release_git_sha} ${release_version}
  divider

  printf "3. Start create_release_draft PipelineRun\n"
  print_create_draft_release_command ${release_git_sha} ${release_version} ${previous_release_version}
  divider

  print_component_versions
}

function print_pipeline_start_command() {
  source ${config_file_path}

  local release_git_sha=${1}; shift
  local release_version=${1}

  start_command="
tkn --context=dogfooding pipeline start operator-release \\
  --serviceaccount=release-right-meow \\
  --param=gitRevision=\"${release_git_sha}\" \\
  --param=versionTag=\"${release_version}\" \\
  --param=TektonCDPipelinesVersion=${TEKTON_PIPELINE_VERSION} \\
  --param=TektonCDTriggersVersion=${TEKTON_TRIGGERS_VERSION} \\
  --param=TektonCDDashboardVersion=${TEKTON_DASHBOARD_VERSION} \\
  --param=TektonCDResultsVersion=${TEKTON_RESULTS_VERSION} \\
  --param=TektonCDChainsVersion=${TEKTON_CHAINS_VERSION} \\
  --param=TektonCDHubVersion=${TEKTON_HUB_VERSION} \\
  --param=PACVersion=${PAC_VERSION} \\
  --param=serviceAccountPath=release.json \\
  --param=releaseBucket=gs://tekton-releases/operator \\
  --param=imageRegistry=gcr.io \\
  --param=imageRegistryPath=tekton-releases  \\
  --param=releaseAsLatest=true \\
  --param=platforms=linux/amd64,linux/arm64,linux/s390x,linux/ppc64le \\
  --param=kubeDistros=\"kubernetes openshift\" \\
  --param=package=github.com/tektoncd/operator \\
  --workspace name=release-secret,secret=release-secret \\
  --workspace name=workarea,volumeClaimTemplateFile=workspace-template.yaml \\
  --timeout 2h0m0s"
  printf "${start_command}\n"
}

function workspace_template_create_command() {
  workspace_template="
cat <<EOF > workspace-template.yaml
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF\n"

  printf "${workspace_template}\n"
}

function print_create_draft_release_command() {
  local release_git_sha=${1}; shift
  local release_version=${1}; shift
  local previous_release_version=${1}

  create_release_draft="
tkn --context dogfooding pipeline start \\
  -p package="tektoncd/operator" \\
  -p git-revision="${release_git_sha}" \\
  -p release-tag="${release_version}" \\
  -p previous-release-tag="${previous_release_version}" \\
  -p release-name="" \\
  -p bucket="gs://tekton-releases/operator" \\
  -p rekor-uuid="" \\
  --workspace name=shared,volumeClaimTemplateFile=workspace-template.yaml \\
  --workspace name=credentials,secret=release-secret \\
  release-draft"

  printf "${create_release_draft}\n"

}




