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

# This script calls out to scripts in tektoncd/plumbing to setup a cluster
# and deploy Tekton Pipelines to it for running integration tests.
set -e

source $(dirname $0)/e2e-common.sh

# Script entry point.
TARGET=${TARGET:-kubernetes}
# In case if KUBECONFIG variable is specified, it will be used for `go test`
KUBECONFIG=${KUBECONFIG:-"${HOME}/.kube/config"}
KUBECONFIG_PARAM=${KUBECONFIG:+"--kubeconfig $KUBECONFIG"}

E2E_SKIP_CLUSTER_CREATION=${E2E_SKIP_CLUSTER_CREATION:="false"}
E2E_SKIP_OPERATOR_INSTALLATION=${E2E_SKIP_OPERATOR_INSTALLATION="false"}

echo "Running tests on ${TARGET}"

header "Provision a cluster"
if [ "${E2E_SKIP_CLUSTER_CREATION}" != "true" ]; then
    initialize $@
fi

header "Setting up environment"
if [ "${E2E_SKIP_OPERATOR_INSTALLATION}" != "true" ]; then
    install_operator_resources $@
fi

failed=0
tektonconfig_ready_wait

header "Running Go e2e tests"
go_test_e2e -timeout=40m ./test/e2e/common ${KUBECONFIG_PARAM} || failed=1
go_test_e2e -timeout=20m ./test/e2e/${TARGET} ${KUBECONFIG_PARAM} || failed=1

(( failed )) && fail_test
success
