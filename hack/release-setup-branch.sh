#!/usr/bin/env bash

set -u

source ./hack/release-common.sh

OPERATOR_RELEASE_VERSION=${1}
set_version_label ${OPERATOR_RELEASE_VERSION}
