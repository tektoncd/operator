#!/usr/bin/env bash

# Copyright 2019 The Tekton Authors
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

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

# Namespace used for tests
readonly TEST_NAMESPACE="operator-tests"

function tekton_setup() {
  header "Installing Tekton Operator"
  kubectl create namespace $TEST_NAMESPACE
  kubectl apply -n $TEST_NAMESPACE -f deploy/crds/*_crd.yaml
  ko apply -n $TEST_NAMESPACE -f config/
  wait_until_pods_running $TEST_NAMESPACE
}

function tekton_teardown() {
  header "Removing Tekton Operator"
  kubectl delete --ignore-not-found Config cluster
  ko apply -n $TEST_NAMESPACE -f config/
  kubectl delete -n $TEST_NAMESPACE -f deploy/crds/*_crd.yaml
  kubectl delete all --all --ignore-not-found --now --timeout 60s -n $TEST_NAMESPACE
  kubectl delete --ignore-not-found --now --timeout 300s namespace $TEST_NAMESPACE
}

# Script entry point.

initialize $@

header "Running operator-sdk test"

export GO111MODULE=on
operator-sdk test local ./test/e2e  \
  --up-local --namespace operators \
  --debug  \
  --verbose || fail_test
success
