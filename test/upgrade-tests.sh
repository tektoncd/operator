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

# Upgrade scenario B:
#   1. Install latest released operator
#   2. Create workloads and wait for success
#   3. Upgrade to current HEAD (with an explicit VERSION bump)
#   4. Assert pre/post upgrade versions match the new VERSION
#   5. Re-check that pre-upgrade workloads survived and new workloads succeed
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

header "Create pre-upgrade workloads"
create_pre_upgrade_workloads

header "Upgrade to current HEAD"
set_upgrade_operator_version
install_operator_resources
tektonconfig_ready_wait

header "Assert upgrade versions"
assert_upgrade_versions

header "Re-check workloads after upgrade"
verify_workloads_after_upgrade

success
