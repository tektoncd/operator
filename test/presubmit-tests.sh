#!/usr/bin/env bash

# Copyright 2018 The Knative Authors
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

# Markdown linting failures don't show up properly in Gubernator resulting
# in a net-negative contributor experience.
export DISABLE_MD_LINTING=1
export GO111MODULE=on

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/presubmit-tests.sh



unit_tests() {
 :
}

execute() {
  echo "$@"
  $@
}

build_tests() {
  execute operator-sdk build gcr.io/tekton-nightly/tektoncd-operator
}

install_operator_sdk() {
  local sdk_rel="v0.9.0"
  curl -JL \
    https://github.com/operator-framework/operator-sdk/releases/download/${sdk_rel}/operator-sdk-${sdk_rel}-x86_64-linux-gnu \
    -o /usr/bin/operator-sdk
  chmod +x /usr/bin/operator-sdk
}

extra_initialization() {
  echo "Running as $(whoami) on $(hostname) under $(pwd) dir"

  install_operator_sdk
  echo ">> operator sdk version"
  operator-sdk version
}

integration_tests() {
  operator-sdk version

  echo "ls $(pwd)"
  ls

  # HACK: -mod=vendor fails to build
  rm -rf vendor
  execute operator-sdk test local ./test/e2e  \
    --up-local --namespace operators \
    --debug  \
    --verbose
}

# We use the default build, unit and integration test runners.

main "$@"
