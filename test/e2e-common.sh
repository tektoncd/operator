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

# This script runs the presubmit tests; it is started by prow for each PR.
# For convenience, it can also be executed manually.
# Running the script without parameters, or with the --all-tests
# flag, causes all tests to be executed, in the right order.
# Use the flags --build-tests, --unit-tests and --integration-tests
# to run a specific set of tests.
set -e

# Helper functions for E2E tests.

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

function install_operator_resources() {

  echo :Payload Targets:
  echo Pipelines: ${TEKTON_PIPELINE_VERSION}
  echo Triggers: ${TEKTON_TRIGGERS_VERSION}
  echo Chains: ${TEKTON_CHAINS_VERSION}
  echo Hub: ${TEKTON_HUB_VERSION}
  if [[ ${TARGET} != "openshift" ]]; then
    echo Results: ${TEKTON_RESULTS_VERSION}
    echo Dashboard: ${TEKTON_DASHBOARD_VERSION}
  fi
  echo '------------------------------'

  echo ">> Deploying Tekton Operator Resources"

  # Allow Go to automatically download the required toolchain version from go.mod
  export GOTOOLCHAIN=auto
  make KO_BIN=$(which ko) KUSTOMIZE_BIN=$(which kustomize) TARGET=${TARGET:-kubernetes} apply || fail_test "Tekton Operator installation failed"

  # Wait for pods to be running in the namespaces we are deploying to
  local operator_namespace=$(get_operator_namespace)
  wait_until_pods_running ${operator_namespace} || fail_test "Tekton Operator controller did not come up"
}

function tektonconfig_ready_wait() {
  echo "Wait for controller to start and create TektonConfig"
  TEKTONCONFIG_READY=False
  until [[ "${TEKTONCONFIG_READY}" = "True" ]]; do
    echo waiting for TektonConfig config Ready status
    sleep 5
    if is_tektonconfig_cr_created && is_tektonconfig_cr_uptodate && is_tektonconfig_cr_ready; then
      TEKTONCONFIG_READY=True
    fi
  done
  echo "TektonConfig config Ready: True"
}

function is_tektonconfig_cr_created() {
  kubectl get TektonConfig config > /dev/null 2>&1
}

function is_tektonconfig_cr_uptodate() {
  cr_status_version=$(current_tektonconfig_version)
  expected_version=$(version_from_info_cm)
  [[ ${cr_status_version} == ${expected_version} ]]
}

function is_tektonconfig_cr_ready() {
  ready_status=$(kubectl get tektonconfig config -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
  [[ ${ready_status} == "True" ]]
}

function current_tektonconfig_version() {
  tektonconfig_version_from_label
  # TODO: Read version from status instead of label
  # reading from status is flaky during upgrade
  # tektonconfig_version_from_status
}

function tektonconfig_version_from_label() {
  label_key='operator.tekton.dev/release-version'
  kubectl get tektonconfig config  -o yaml | grep ${label_key} | head -n 1 | tr -d ' ' | cut -d ':' -f 2
}

tektonconfig_version_from_status() {
  kubectl get tektonconfig config -o jsonpath='{.status.version}'
}

function version_from_info_cm() {
  local operator_namespace=$(get_operator_namespace)
  kubectl get configmap tekton-operator-info -n ${operator_namespace} -o jsonpath="{.data.version}"
}

function latest_released_version() {
  repo_url='https://api.github.com/repos/tektoncd/operator/releases'
  curl -sSL ${repo_url} | jq -r '.[].tag_name' | sort -Vr | head -n 1
}

function set_release_file_name() {
  local platform=${1}
  local name='release.notags.yaml'
  if [[ ${platform} != "kubernetes" ]]; then
    name=${platform}-${name}
  fi
  echo ${name}
}

function get_operator_namespace() {
  # TODO: parameterize namespace, operator can run in a namespace different from the namespace where tektonpipelines is installed
  local operator_namespace="tekton-operator"
  [[ "${TARGET}" == "openshift" ]] && operator_namespace="openshift-operators"
  echo ${operator_namespace}
}

function install_latest_released_version() {
  local platform=${1}
  version=$(latest_released_version)
  release_file_name=$(set_release_file_name ${platform})
  release_manifest_url="https://github.com/tektoncd/operator/releases/download/${version}/${release_file_name}"
  echo "latest_release_url: ${release_manifest_url}"
  kubectl apply -f ${release_manifest_url}

  # Wait for pods to be running in the namespaces we are deploying to
  local operator_namespace=$(get_operator_namespace)
  wait_until_pods_running ${operator_namespace} || fail_test "Tekton Operator controller did not come up"
}
