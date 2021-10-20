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

# Helper functions for E2E tests.

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh
source $(dirname $0)/config.sh

function install_operator_resources() {

  echo :Payload Targets:
  echo Pipelines: ${PIPELINES}
  echo Triggers: ${TRIGGERS}
  if [[ ${TARGET} != "openshift" ]]; then
    echo Results: ${RESULTS}
    echo Dashboard: ${DASHBOARD}
  fi
  echo '------------------------------'

  echo ">> Deploying Tekton Operator Resources"

  make TARGET=${TARGET:-kubernetes} apply || fail_test "Tekton Operator installation failed"

  OPERATOR_NAMESPACE="tekton-operator"
  [[ "${TARGET}" == "openshift" ]] && OPERATOR_NAMESPACE="openshift-operators"

  # Wait for pods to be running in the namespaces we are deploying to
  # TODO: parameterize namespace, operator can run in a namespace different from the namespace where tektonpipelines is installed
  wait_until_pods_running ${OPERATOR_NAMESPACE} || fail_test "Tekton Operator controller did not come up"

  # Make sure that everything is cleaned up in the current namespace.
  for res in tektonpipelines tektontriggers tektondashboards; do
    kubectl delete --ignore-not-found=true ${res}.operator.tekton.dev --all
  done
}
