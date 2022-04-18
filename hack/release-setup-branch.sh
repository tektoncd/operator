#!/usr/bin/env bash

set -u

# useage:
# ./hack/release-setup-branch.sh <release version> <previous release version>
# eg: ./hack/release-setup-branch.sh 0.55.0 0.54.1

SCRIPT_DIR=$(dirname $0)

source ${SCRIPT_DIR}/release-const.sh
source ${SCRIPT_DIR}/release-common.sh
source ${SCRIPT_DIR}/release-freeze-component-versions.sh
source ${SCRIPT_DIR}/release-commands.sh

# read args
declare -r TEKTONCD_OPERATOR_RELEASE_VERSION=${1}; shift
declare -r TEKTONCD_OPERATOR_PREVIOUS_RELEASE_VERSION=${1}; shift
declare -r git_remote_name=${1:-upstream}

remote_exists_or_fail ${git_remote_name}

checkout_release_branch ${git_remote_name} ${TEKTONCD_OPERATOR_RELEASE_VERSION}

# update tasks for openshift
echo updating addon tasks for OpenShift
${SCRIPT_DIR}/openshift/fetch-tektoncd-catalog-tasks.sh

choose_component_versions

set_version_label ${TEKTONCD_OPERATOR_RELEASE_VERSION}

commit_changes ${TEKTONCD_OPERATOR_RELEASE_VERSION}

push_release_branch ${git_remote_name} ${TEKTONCD_OPERATOR_RELEASE_VERSION}
[[ "$?" != 0 ]] && exit 1

declare -r git_commit_sha=$(git rev-parse HEAD)
print_release_commands ${git_commit_sha} ${TEKTONCD_OPERATOR_RELEASE_VERSION} ${TEKTONCD_OPERATOR_PREVIOUS_RELEASE_VERSION} 2>&1 | tee release-setup-branch-${TEKTONCD_OPERATOR_RELEASE_VERSION}.log
divider
echo set up release branch: done
divider