#!/usr/bin/env bash
#
# Download and copy opeartor-sdk to the desired location. The target location is specified by the
# first argument. For instance:
#
# $ install-operator-sdk.sh "bin/operator-sdk"
#

set -e

DEST="${1:-./bin/operator-sdk}"
SDK_VERSION="${SDK_VERSION:-1.37.0}"

OS="${OS:-linux}"
ARCH="${ARCH:-amd64}"

SDK_URL_HOST="${SDK_HOST:-github.com}"
SDK_URL_PATH="${SDK_URL_PATH:-operator-framework/operator-sdk/releases/download}"
SDK_URL="https://${SDK_URL_HOST}/${SDK_URL_PATH}/v${SDK_VERSION}/operator-sdk_${OS}_${ARCH}"

if [ -x ${DEST} ] ; then
    echo "# operator-sdk is already installed at '${DEST}'"
    exit 0
fi

curl --silent --location --output "${DEST}" "${SDK_URL}"
chmod +x "${DEST}"
