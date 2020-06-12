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

initialize $@

header "Running operator-sdk test"

 operator-sdk test local ./test/e2e  \
  --up-local \
  --operator-namespace operators \
  --watch-namespace "" \
  --debug  \
  --verbose || fail_test
success
