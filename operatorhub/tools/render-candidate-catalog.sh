#! /usr/bin/env bash
# Copyright The tektoncd Contributors
#
# SPDX-License-Identifier: Apache-2.0
# render-candidate-catalog.sh creates a file-based OLM catalog containing the following:
set -euo pipefail
echo "Rendering operator catalog from candidate to released"
# Check if CATALOG_DIR is set and exit with an error if not
if [ -z "${CATALOG_DIR:-}" ]; then
echo "CATALOG_DIR is not set."
exit 1
fi
echo "Using CATALOG_DIR: ${CATALOG_DIR}"
# Set paths for the tools
OPM_BIN=${OPM_BIN:-${PWD}/.bin/opm}
SED_BIN=${SED_BIN:-sed}
CSV_VERSION=${CSV_VERSION:-${VERSION}}
USE_HTTP=${USE_HTTP:-false}
# Define paths to channel and bundle specifications for both candidate and released
candidateChannelSpec="${CATALOG_DIR}/candidate/tekton-operator-channel-candidate.json"
candidateBundleSpec="${CATALOG_DIR}/candidate/tekton-operator-bundle.json"
releasedChannelSpec="${CATALOG_DIR}/released/tekton-operator-channel-candidate.json"
releasedBundleSpec="${CATALOG_DIR}/released/tekton-operator-bundle.json"
# Ensure the required directories exist and copy catalog manifests
echo "Copying catalog manifests"
rm -rf "${CATALOG_DIR}/released"
mkdir -p "${CATALOG_DIR}/released"
cp -r "${CATALOG_DIR}/candidate/"* "${CATALOG_DIR}/released/"
# Modify the channel file for the released version using the CSV_VERSION
echo "Rendering candidate operator channel to released channel"
#echo "Current content of channel spec:"
#cat "${candidateChannelSpec}"
# Only modify the entry name, not the channel name
${SED_BIN} -i -E 's|"name": "tekton-operator-latest"|"name": "tektoncd-operator.v'"${CSV_VERSION}"'"|g' "${releasedChannelSpec}"
#echo "Modified content of channel spec:"
#cat "${releasedChannelSpec}"
# Render the OLM content from the candidate bundle image (for local testing/CI)
echo "Rendering the OLM content from the candidate bundle"
${OPM_BIN} render "${BUNDLE_IMG}" --use-http="${USE_HTTP}" > "${releasedBundleSpec}"
echo "Removing existing Dockerfile if it exists"
rm -f "${CATALOG_DIR}/released.Dockerfile"
# Generate the Dockerfile for the released catalog
echo "Generating catalog Dockerfile for released"
${OPM_BIN} generate dockerfile "${CATALOG_DIR}/released"
echo "Done"
