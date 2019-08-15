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

export GO111MODULE=on
source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

# Script entry point.

function install_operator() {
    echo ">> install operator"
    kubectl apply -f deploy/crds/*_crd.yaml
    ko apply -f config/ || fail_test "Operator installation failed"
    wait_until_pods_running default || fail_test "Operator did not come up"
}

function test_teardown() {
    subheader "Tearing down Operator"
    ko delete --ignore-not-found=true -f config/
    kubectl delete -f deploy/crds/*_crd.yaml
    kubectl delete Config cluster
}

initialize $@

header "Setting up environment"

install_operator

header "Running operator-sdk test"

operator-sdk test local ./test/e2e  \
  --up-local --namespace operators \
  --debug  \
  --verbose || fail_test
success
