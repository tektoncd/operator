#!/usr/bin/env bash

# Copyright 2020 The Tekton Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Upgrade test:
#   1. Install latest released operator
#   2. Record/assert release TektonConfig version (baseline)
#   3. Create workloads and wait for success
#   4. Upgrade to current HEAD (with an explicit VERSION bump)
#   5. Assert pre/post upgrade versions match the new VERSION
#   6. Assert component CRs Ready and no InstallerSets on the old release-version
#   7. Re-check that pre-upgrade workloads survived and new workloads succeed
#
# Local (cluster already exists):
#   E2E_SKIP_CLUSTER_CREATION=true KO_DOCKER_REPO=<registry> ./test/upgrade-tests.sh
#
# CI (Kind):
#   ./hack/setup-kind.sh ... --e2e-script ./test/upgrade-tests.sh --e2e-env ./test/e2e-tests-kind.env

source $(dirname $0)/e2e-common.sh
set -ueo pipefail

TARGET=${TARGET:-kubernetes}
E2E_SKIP_CLUSTER_CREATION=${E2E_SKIP_CLUSTER_CREATION:-"false"}
UPGRADE_NS=operator-upgrade-test
UPGRADE_VERSION=${UPGRADE_VERSION:-"upgrade-test-$(git rev-parse --short HEAD)"}
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
RELEASE_VERSION_LABEL_KEY='operator.tekton.dev/release-version'

echo "upgrade test platform: ${TARGET}"
echo "upgrade version: ${UPGRADE_VERSION}"

# create cluster unless skipped (Kind CI sets E2E_SKIP_CLUSTER_CREATION=true)
if [[ "${E2E_SKIP_CLUSTER_CREATION}" != "true" ]]; then
  initialize $@
fi

function set_upgrade_operator_version() {
  local operator_yaml="config/${TARGET}/base/operator.yaml"
  local info_yaml="config/base/config-info.yaml"

  echo ">> Setting operator VERSION to ${UPGRADE_VERSION}"
  # Portable in-place sed (GNU and BSD)
  sed -i.bak "s/value: \"devel\"/value: \"${UPGRADE_VERSION}\"/g" "${operator_yaml}"
  sed -i.bak "s/version: \"devel\"/version: \"${UPGRADE_VERSION}\"/g" "${info_yaml}"
  rm -f "${operator_yaml}.bak" "${info_yaml}.bak"
}

function create_pre_upgrade_workloads() {
  echo ">> Creating pre-upgrade workloads"
  kubectl apply -f "${SCRIPT_DIR}/upgrade/workloads-pre.yaml"
  wait_for_taskrun_success upgrade-echo-pre
}

function wait_for_taskrun_success() {
  local name=${1}
  echo ">> Waiting for TaskRun ${name} to succeed"
  # Succeeded condition is True when the TaskRun completed successfully
  if ! kubectl wait --for=condition=Succeeded=True "taskrun/${name}" -n "${UPGRADE_NS}" --timeout=10m; then
    echo "TaskRun ${name} did not succeed:"
    kubectl get taskrun "${name}" -n "${UPGRADE_NS}" -o yaml || true
    fail_test "TaskRun ${name} failed or timed out"
  fi
}

# Record the release operator version from the cluster and assert it looks like a
# real release install (not already our upgrade VERSION). Prevents a silent
# fresh-install of HEAD from passing as an "upgrade".
function record_and_assert_release_baseline() {
  echo ">> Recording pre-upgrade TektonConfig version baseline"
  PRE_UPGRADE_VERSION=$(current_tektonconfig_version)
  PRE_UPGRADE_VERSION=$(echo -n "${PRE_UPGRADE_VERSION}" | tr -d '[:space:]')
  export PRE_UPGRADE_VERSION

  if [[ -z "${PRE_UPGRADE_VERSION}" ]]; then
    fail_test "TektonConfig release-version label is empty after installing the release"
  fi
  if [[ "${PRE_UPGRADE_VERSION}" == "${UPGRADE_VERSION}" ]]; then
    fail_test "pre-upgrade version already equals UPGRADE_VERSION (${UPGRADE_VERSION}); not a release→HEAD upgrade"
  fi

  local info_version
  info_version=$(version_from_info_cm)
  info_version=$(echo -n "${info_version}" | tr -d '[:space:]')
  if [[ "${PRE_UPGRADE_VERSION}" != "${info_version}" ]]; then
    fail_test "TektonConfig version (${PRE_UPGRADE_VERSION}) != tekton-operator-info (${info_version})"
  fi

  echo "pre-upgrade baseline version: ${PRE_UPGRADE_VERSION} (release tag: ${INSTALLED_RELEASE_TAG:-unknown})"
}

function assert_component_cr_ready() {
  local kind=${1}
  local name=${2}
  local ready
  if ! kubectl get "${kind}" "${name}" >/dev/null 2>&1; then
    fail_test "expected ${kind}/${name} to exist after upgrade"
  fi
  ready=$(kubectl get "${kind}" "${name}" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
  if [[ "${ready}" != "True" ]]; then
    kubectl get "${kind}" "${name}" -o yaml || true
    fail_test "${kind}/${name} not Ready after upgrade (status=${ready})"
  fi
  echo "${kind}/${name} Ready=True"
}

function assert_components_ready_after_upgrade() {
  echo ">> Asserting component CRs are Ready after upgrade"
  assert_component_cr_ready tektonpipelines pipeline
  assert_component_cr_ready tektontriggers trigger
  assert_component_cr_ready tektonchains chain
  if [[ "${TARGET}" != "openshift" ]]; then
    assert_component_cr_ready tektondashboards dashboard
  fi
}

function assert_no_installersets_on_old_release() {
  local old_version=${PRE_UPGRADE_VERSION}
  echo ">> Asserting no TektonInstallerSets remain on release-version=${old_version}"
  local leftover
  leftover=$(kubectl get tektoninstallersets \
    -l "${RELEASE_VERSION_LABEL_KEY}=${old_version}" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' || true)
  if [[ -n "${leftover}" ]]; then
    echo "InstallerSets still labeled with old release-version ${old_version}:"
    echo "${leftover}"
    kubectl get tektoninstallersets -l "${RELEASE_VERSION_LABEL_KEY}=${old_version}" -o wide || true
    fail_test "InstallerSets still carry old release-version ${old_version}"
  fi
  echo "no InstallerSets with ${RELEASE_VERSION_LABEL_KEY}=${old_version}"
}

function assert_upgrade_versions() {
  local expected=${UPGRADE_VERSION}
  echo ">> Asserting TektonConfig upgrade versions == ${expected}"

  local pre
  local post
  pre=$(kubectl get tektonconfig config -o jsonpath="{.status.annotations.operator\.tekton\.dev/pre-upgrade-version}")
  post=$(kubectl get tektonconfig config -o jsonpath="{.status.annotations.operator\.tekton\.dev/post-upgrade-version}")

  if [[ "${pre}" != "${expected}" ]]; then
    fail_test "pre-upgrade-version: want ${expected}, got ${pre}"
  fi
  if [[ "${post}" != "${expected}" ]]; then
    fail_test "post-upgrade-version: want ${expected}, got ${post}"
  fi
  echo "pre-upgrade-version and post-upgrade-version match ${expected}"
}

function verify_workloads_after_upgrade() {
  echo ">> Verifying pre-upgrade Task still exists"
  kubectl get task upgrade-echo -n "${UPGRADE_NS}" >/dev/null \
    || fail_test "Task upgrade-echo missing after upgrade"

  echo ">> Verifying pre-upgrade TaskRun still exists and succeeded"
  local pre_cond
  pre_cond=$(kubectl get taskrun upgrade-echo-pre -n "${UPGRADE_NS}" \
    -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}')
  if [[ "${pre_cond}" != "True" ]]; then
    fail_test "pre-upgrade TaskRun no longer Succeeded after upgrade (status=${pre_cond})"
  fi

  echo ">> Creating post-upgrade TaskRun"
  kubectl apply -f "${SCRIPT_DIR}/upgrade/workloads-post.yaml"
  wait_for_taskrun_success upgrade-echo-post
}

header "Install latest released operator"
install_latest_released_version ${TARGET}
tektonconfig_ready_wait

header "Assert pre-upgrade release baseline"
record_and_assert_release_baseline

header "Create pre-upgrade workloads"
create_pre_upgrade_workloads

header "Upgrade to current HEAD"
set_upgrade_operator_version
install_operator_resources
tektonconfig_ready_wait

header "Assert upgrade versions"
assert_upgrade_versions

header "Assert components Ready and InstallerSets upgraded"
assert_components_ready_after_upgrade
assert_no_installersets_on_old_release

header "Re-check workloads after upgrade"
verify_workloads_after_upgrade

success
