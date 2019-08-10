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

declare -r SCRIPT_PATH=$(readlink -f "$0")
declare -r SCRIPT_DIR=$(cd $(dirname "$SCRIPT_PATH") && pwd)

# ensure the current working dir is the root of the project
cd $SCRIPT_DIR/../

# This script needs helper functions from tektoncd/plumbing/scripts and
# although github.com/tektoncd/plumbing is added as a go mod dependency,
# the package may not exists when the test is running, so, it ensure the
# package is available, run go mod download which downloads the packages to
# $GOPATH/pkg/mod/repo/path@version
# GOPROXY ensures the downloads is faster

export GO111MODULE=on
export DISABLE_MD_LINTING=1
source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/presubmit-tests.sh

unit_tests() {
 :
}

build_tests() {
  header "Running ko build"
  ko publish --local github.com/tektoncd/operator/cmd/manager
}

main "$@"
