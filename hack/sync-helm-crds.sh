#!/usr/bin/env bash

# Copyright 2025 The Tekton Authors
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

# This script syncs generated CRDs from config/base/generated-crds/ into
# charts/tekton-operator/templates/ (Helm chart CRDs).
#
# kustomize consumes config/base/generated-crds/*.yaml directly (see
# config/base/kustomization.yaml, config/kubernetes/base/kustomization.yaml,
# config/openshift/base/kustomization.yaml) — there is no separate manual copy
# to keep in sync there. See commit 1ae0906b3 ("Cleanup manual CRDs and use
# generated CRDs in kustomize"), which removed those manual config/*_crd.yaml
# files; this script must not recreate them.
#
# Prerequisites: Run `make generate-crds` first (or use `make sync-helm-crds` which calls this).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATED_DIR="${REPO_ROOT}/config/base/generated-crds"
HELM_DIR="${REPO_ROOT}/charts/tekton-operator/templates"

# inject_labels adds the standard labels after the metadata.name line in a generated CRD
inject_labels() {
  local input_file="$1"
  local tmpfile
  tmpfile=$(mktemp)
  while IFS= read -r line; do
    echo "$line" >> "$tmpfile"
    # After the "  name: ..." line under metadata, inject labels
    if echo "$line" | grep -qE '^  name: .*\.operator\.tekton\.dev$'; then
      echo '  labels:' >> "$tmpfile"
      echo '    version: "devel"' >> "$tmpfile"
      echo '    operator.tekton.dev/release: "devel"' >> "$tmpfile"
    fi
  done < "$input_file"
  cat "$tmpfile"
  rm -f "$tmpfile"
}

# strip_leading_separator removes the leading "---" from controller-gen output
strip_leading_separator() {
  sed '1{/^---$/d;}'
}

# strip_int_formats removes "format: int32" and "format: int64" lines from CRD schemas.
# Kubernetes does not recognize these OpenAPI format strings and emits warnings on apply;
# dropping them is safe because the Kubernetes API server performs no validation based on them.
strip_int_formats() {
  grep -v '^\s*format: int\(32\|64\)$'
}

# assemble_helm_crds assembles a Helm chart CRD file from multiple generated CRDs
assemble_helm_crds() {
  local output_file="$1"
  local condition="$2"
  shift 2
  local crd_files=("$@")

  echo "${condition}" > "$output_file"
  for crd_file in "${crd_files[@]}"; do
    echo "---" >> "$output_file"
    inject_labels "${GENERATED_DIR}/${crd_file}" | strip_leading_separator | strip_int_formats >> "$output_file"
  done
  echo '{{- end -}}' >> "$output_file"

  echo "  Updated: ${output_file}"
}

echo "Syncing generated CRDs..."
echo ""

# Step 1: Assemble Helm chart CRDs
echo "Step 1: Assembling Helm chart CRD files..."

assemble_helm_crds \
  "${HELM_DIR}/kubernetes-crds.yaml" \
  '{{- if (and (not .Values.openshift.enabled) .Values.installCRDs) -}}' \
  "operator.tekton.dev_manualapprovalgates.yaml" \
  "operator.tekton.dev_openshiftpipelinesascodes.yaml" \
  "operator.tekton.dev_tektonchains.yaml" \
  "operator.tekton.dev_tektonconfigs.yaml" \
  "operator.tekton.dev_tektondashboards.yaml" \
  "operator.tekton.dev_tektoninstallersets.yaml" \
  "operator.tekton.dev_tektonpipelines.yaml" \
  "operator.tekton.dev_tektonresults.yaml" \
  "operator.tekton.dev_tektontriggers.yaml" \
  "operator.tekton.dev_tektonpruners.yaml" \
  "operator.tekton.dev_tektonschedulers.yaml" \
  "operator.tekton.dev_tektonmulticlusterproxyaaes.yaml" \
  "operator.tekton.dev_syncerservices.yaml"

assemble_helm_crds \
  "${HELM_DIR}/openshift-crds.yaml" \
  '{{- if (and .Values.openshift.enabled .Values.installCRDs) -}}' \
  "operator.tekton.dev_manualapprovalgates.yaml" \
  "operator.tekton.dev_openshiftpipelinesascodes.yaml" \
  "operator.tekton.dev_tektonaddons.yaml" \
  "operator.tekton.dev_tektonchains.yaml" \
  "operator.tekton.dev_tektonconfigs.yaml" \
  "operator.tekton.dev_tektoninstallersets.yaml" \
  "operator.tekton.dev_tektonpipelines.yaml" \
  "operator.tekton.dev_tektonresults.yaml" \
  "operator.tekton.dev_tektontriggers.yaml" \
  "operator.tekton.dev_tektonpruners.yaml" \
  "operator.tekton.dev_tektonschedulers.yaml" \
  "operator.tekton.dev_tektonmulticlusterproxyaaes.yaml" \
  "operator.tekton.dev_syncerservices.yaml"

# Step 2: Validate CRD sizes (etcd has a 256KB object size limit)
echo "Step 2: Validating CRD sizes..."

MAX_SIZE=262144 # 256KB
FAILED=0
for file in "${GENERATED_DIR}"/*.yaml; do
  # Calculate JSON size using yq to simulate the size of the applied configuration
  SIZE=$(yq -o=json -I=0 . "$file" | wc -c | tr -d ' ')
  echo "  $(basename "$file"): ${SIZE} bytes"
  if [ "$SIZE" -gt "$MAX_SIZE" ]; then
    echo "  ERROR: $(basename "$file") JSON size (${SIZE} bytes) exceeds the limit of ${MAX_SIZE} bytes (256KB)."
    FAILED=1
  fi
done

if [ "$FAILED" -eq 1 ]; then
  echo ""
  echo "FAILED: One or more CRDs exceed the 256KB JSON size limit."
  echo "Consider adding +kubebuilder:validation:Schemaless to large embedded types."
  exit 1
fi

echo ""
echo "Done! CRDs synced and validated successfully."
