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

# Markdown linting failures don't show up properly in Gubernator resulting
# in a net-negative contributor experience.
export DISABLE_MD_LINK_CHECK=1
export DISABLE_MD_LINTING=1

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/presubmit-tests.sh

function post_build_tests() {
    golangci-lint run
}

function build_tests() {
    # TODO add build tests for operator, since the default build tests fail on checking the bundled yamls.
    echo "Skip all the build tests for now"
}

install_operator_sdk() {
  local sdk_rel="v0.17.0"
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

# We use the default build, unit and integration test runners.

main $@
