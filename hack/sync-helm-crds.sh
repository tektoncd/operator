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

# This script syncs generated CRDs from config/base/generated-crds/ into:
#   1. config/base/, config/kubernetes/base/, config/openshift/base/ (kustomize source-of-truth)
#   2. charts/tekton-operator/templates/ (Helm chart CRDs)
#
# Prerequisites: Run `make generate-crds` first (or use `make sync-helm-crds` which calls this).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATED_DIR="${REPO_ROOT}/config/base/generated-crds"
BASE_DIR="${REPO_ROOT}/config/base"
K8S_DIR="${REPO_ROOT}/config/kubernetes/base"
OPENSHIFT_DIR="${REPO_ROOT}/config/openshift/base"
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

# write_config_crd writes a CRD to the appropriate config/ directory with copyright header and labels
write_config_crd() {
  local src_file="$1"
  local dest_dir="$2"
  local dest_file="$3"

  cat > "${dest_dir}/${dest_file}" <<'HEADER'
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

HEADER

  inject_labels "$src_file" | strip_leading_separator >> "${dest_dir}/${dest_file}"
  echo "  Updated: ${dest_dir}/${dest_file}"
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
    inject_labels "${GENERATED_DIR}/${crd_file}" | strip_leading_separator >> "$output_file"
  done
  echo '{{- end -}}' >> "$output_file"

  echo "  Updated: ${output_file}"
}

echo "Syncing generated CRDs..."
echo ""

# Step 1: Update config/ directories
echo "Step 1: Updating config/ CRD files..."

# base CRDs (common to both kubernetes and openshift)
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_manualapprovalgates.yaml"         "$BASE_DIR"      "300-operator_v1alpha1_manualapprovalgate_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonchains.yaml"                "$BASE_DIR"      "300-operator_v1alpha1_chain_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonconfigs.yaml"               "$BASE_DIR"      "300-operator_v1alpha1_config_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonhubs.yaml"                  "$BASE_DIR"      "300-operator_v1alpha1_hub_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektoninstallersets.yaml"          "$BASE_DIR"      "300-operator_v1alpha1_installer_set_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonpipelines.yaml"             "$BASE_DIR"      "300-operator_v1alpha1_pipeline_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonresults.yaml"               "$BASE_DIR"      "300-operator_v1alpha1_result_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektontriggers.yaml"              "$BASE_DIR"      "300-operator_v1alpha1_trigger_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonpruners.yaml"               "$BASE_DIR"      "300-operator_v1alpha1_pruner_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonschedulers.yaml"            "$BASE_DIR"      "300-operator_v1alpha1_scheduler_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonmulticlusterproxyaaes.yaml" "$BASE_DIR"      "300-operator_v1alpha1_multiclusterproxyaae_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_syncerservices.yaml"              "$BASE_DIR"      "300-operator_v1alpha1_syncerservice_crd.yaml"

# kubernetes-only CRDs
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektondashboards.yaml"            "$K8S_DIR"       "300-operator_v1alpha1_dashboard_crd.yaml"

# openshift-only CRDs
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_tektonaddons.yaml"                "$OPENSHIFT_DIR" "300-operator_v1alpha1_addon_crd.yaml"
write_config_crd "${GENERATED_DIR}/operator.tekton.dev_openshiftpipelinesascodes.yaml"   "$OPENSHIFT_DIR" "300-operator_v1alpha1_openshiftpipelinesascode_crd.yaml"

echo ""

# Step 2: Assemble Helm chart CRDs
echo "Step 2: Assembling Helm chart CRD files..."

assemble_helm_crds \
  "${HELM_DIR}/kubernetes-crds.yaml" \
  '{{- if (and (not .Values.openshift.enabled) .Values.installCRDs) -}}' \
  "operator.tekton.dev_manualapprovalgates.yaml" \
  "operator.tekton.dev_openshiftpipelinesascodes.yaml" \
  "operator.tekton.dev_tektonchains.yaml" \
  "operator.tekton.dev_tektonconfigs.yaml" \
  "operator.tekton.dev_tektondashboards.yaml" \
  "operator.tekton.dev_tektonhubs.yaml" \
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
  "operator.tekton.dev_tektonhubs.yaml" \
  "operator.tekton.dev_tektoninstallersets.yaml" \
  "operator.tekton.dev_tektonpipelines.yaml" \
  "operator.tekton.dev_tektonresults.yaml" \
  "operator.tekton.dev_tektontriggers.yaml" \
  "operator.tekton.dev_tektonpruners.yaml" \
  "operator.tekton.dev_tektonschedulers.yaml" \
  "operator.tekton.dev_tektonmulticlusterproxyaaes.yaml" \
  "operator.tekton.dev_syncerservices.yaml"

# Step 3: Validate CRD sizes (etcd has a 256KB object size limit)
echo "Step 3: Validating CRD sizes..."

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
