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

# Backup the vendor directory, since operator-sdk commands cannot find the modules available under vendor.
mv vendor vendor_backup

# This script needs helper functions from tektoncd/plumbing/scripts and
# although github.com/tektoncd/plumbing is added as a go mod dependency,
# the package may not exists when the test is running, so, it ensure the
# package is available, run go mod download which downloads the packages to
# $GOPATH/pkg/mod/repo/path@version
# GOPROXY ensures the downloads is faster

export GO111MODULE=on
export GOPROXY="https://proxy.golang.org"
go mod vendor

export DISABLE_MD_LINTING=1
source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/presubmit-tests.sh

unit_tests() {
 :
}

build_tests() {
  header "Running operator-sdk build"
  operator-sdk build gcr.io/tekton-nightly/tektoncd-operator
}

install_operator_sdk() {
  local sdk_rel="v0.10.0"
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


main "$@"

# Restore the vendor directory.
#mv vendor_backup vendor
