#!/usr/bin/env bash
set -u

# usage:
# ./hack/release-setup-branch.sh <old release version> <new release version>
# eg: ./hack/release-setup-branch.sh devel 0.55.0
# eg: ./hack/release-setup-branch.sh 0.55.0 0.55.1

SCRIPT_DIR=$(dirname "$0")
source "${SCRIPT_DIR}/release.sh"

declare -r TEKTONCD_OPERATOR_OLD_RELEASE_VERSION=${1}; shift
declare -r TEKTONCD_OPERATOR_RELEASE_VERSION=${1}; shift
declare -r git_remote_name=${1:-upstream}

checkout_release_branch "${git_remote_name}" "${TEKTONCD_OPERATOR_RELEASE_VERSION}"
set_version_label "${TEKTONCD_OPERATOR_OLD_RELEASE_VERSION}" "${TEKTONCD_OPERATOR_RELEASE_VERSION}"

# Check if there are changes before committing
if [[ -n $(git status --porcelain) ]]; then
  commit_changes "${TEKTONCD_OPERATOR_RELEASE_VERSION}"
else
  echo "No changes detected, skipping commit."
fi

# Determine if it's a patch release
IFS='.' read -r major minor patch <<< "${TEKTONCD_OPERATOR_RELEASE_VERSION}"

if [[ "${patch}" == "0" ]]; then
  echo "Minor release detected: ${TEKTONCD_OPERATOR_RELEASE_VERSION}. Pushing release branch..."
  push_release_branch "${git_remote_name}" "${TEKTONCD_OPERATOR_RELEASE_VERSION}"
else
  echo "Patch release detected: ${TEKTONCD_OPERATOR_RELEASE_VERSION}."
  # Ask user to create PR
  base_branch="release-v${major}.${minor}.x"
  echo "Please create a PR manually instead of pushing directly."

  echo "Before creating the PR, make sure to push the base branch (${base_branch}) to your fork if it isn't already there."
  
  echo "Run the following command to create the PR manually:"
  echo "gh pr create --base ${base_branch} --head release-v${TEKTONCD_OPERATOR_RELEASE_VERSION} --title 'Patch release ${TEKTONCD_OPERATOR_RELEASE_VERSION}' --body 'This PR contains the changes for the patch release ${TEKTONCD_OPERATOR_RELEASE_VERSION}.'"
fi
