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

export DISABLE_MD_LINTING=1
export DISABLE_MD_LINK_CHECK=1
export DISABLE_YAML_LINTING=1

source $(git rev-parse --show-toplevel)/vendor/github.com/tektoncd/plumbing/scripts/presubmit-tests.sh

function check_go_lint() {
    header "Testing if golint has been done"

    # deadline of 10m, and show all the issues
    golangci-lint -j 1 --color=never --timeout=10m run --verbose ./...

    if [[ $? != 0 ]]; then
        results_banner "Go Lint" 1
        exit 1
    fi

    results_banner "Go Lint" 0
}

function check_yaml_lint() {
    header "Testing if yamllint has been done"

    local YAML_FILES=$(find . \( -path ./vendor -prune \) -o \( -type d -name testdata -prune \) -o -type f -regex ".*y[a]ml" -print)
    yamllint -c .yamllint ${YAML_FILES}

    if [[ $? != 0 ]]; then
        results_banner "YAML Lint" 1
        exit 1
    fi

    results_banner "YAML Lint" 0
}

function post_build_tests() {
  # golangci-lint is executed now via GitHub actions
  # check_go_lint
  check_yaml_lint
}

# We use the default build, unit and integration test runners.

main $@
