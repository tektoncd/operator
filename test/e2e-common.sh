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

# Helper functions for E2E tests.

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

# Namespace of Tekton Operator
readonly OPERATOR_NAMESPACE="tekton-operator"

function teardown() {
    subheader "Tearing down Tekton operator"
    ko delete --ignore-not-found=true -f config/
    wait_until_object_does_not_exist namespace ${OPERATOR_NAMESPACE}
}

function install_operator() {
  echo ">> Deploying Tekton Operator"
  ko apply -f config/ || fail_test "Build operator installation failed"
  verify_operator_installation
  echo ">> Creating the cluster-admin role to the operator service account"
  kubectl create clusterrolebinding tekton-operator-cluster-admin --clusterrole cluster-admin --serviceaccount tekton-operator:tekton-operator
}

function verify_operator_installation() {
  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running ${OPERATOR_NAMESPACE} || fail_test "Tekton Operator did not come up"
}
