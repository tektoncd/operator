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

# This script calls out to scripts in tektoncd/plumbing to setup a cluster
# and deploy Tekton Pipelines to it for running integration tests.
declare -r SCRIPT_PATH=$(readlink -f "$0")
declare -r SCRIPT_DIR=$(cd $(dirname "$SCRIPT_PATH") && pwd)

# ensure the current working dir is the root of the project
cd $SCRIPT_DIR/../

# Backup the vendor directory, since operator-sdk commands cannot find the modules available under vendor.
#mv vendor vendor_backup

# This script needs helper functions from tektoncd/plumbing/scripts and
# although github.com/tektoncd/plumbing is added as a go mod dependency,
# the package may not exists when the test is running, so, it ensure the
# package is available, run go mod download which downloads the packages to
# $GOPATH/pkg/mod/repo/path@version
# GOPROXY ensures the downloads is faster

export GO111MODULE=on
export GOFLAGS=-mod=vendor
#export GOPROXY="https://proxy.golang.org"
#go mod vendor

#plumbing_dir_name="$(grep github.com/tektoncd/plumbing go.sum |
#  grep -v go.mod |
#  head -1 | awk '{ print $1 "@" $2 }')"
#plumbing_path="$(go env GOPATH)/pkg/mod/$plumbing_dir_name"
#source "$plumbing_path/scripts/e2e-tests.sh"
source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

# Script entry point.

initialize $@

header "Running operator-sdk test"

operator-sdk test local ./test/e2e  \
  --up-local --namespace operators \
  --debug  \
  --verbose || fail_test
success

# Restore the vendor directory.
#mv vendor_backup vendor
