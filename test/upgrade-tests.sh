#!/usr/bin/env bash

# usage:
# local testing:
# E2E_SKIP_CLUSTER_CREATION=true ./test/upgrade-tests.sh # assumption: test cluster already exists
#
# in upstream ci:
# ./test/upgrade-tests.sh if test cluster already exists

source $(dirname $0)/e2e-common.sh
set -ue

TARGET=${TARGET:-kubernetes}
echo upgrade test platform: ${TARGET}

# create cluster (upstream gke cluster will be provisioned
# set E2E_SKIP_CLUSTER_CREATION=true ./test/upgrade-tests.sh if test cluster already exists
[[ -z ${E2E_SKIP_CLUSTER_CREATION} ]] && initialize $@

# install latest released version
install_latest_released_version ${TARGET}
tektonconfig_ready_wait

# run e2e tests (explicitly skip cluster creation as cluster is already created)
# running e2e tests will install the operator from current code base,
# causing an upgrade. Once Tektonconfig config is Ready: True
# e2e tests will be run
E2E_SKIP_CLUSTER_CREATION=true $(dirname $0)/e2e-tests.sh
