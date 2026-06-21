#!/usr/bin/env bash
set -e -u -o pipefail

# Images to update
declare -A IMAGES=(
  ["buildah"]="registry.redhat.io/rhel9/buildah"
  ["kn"]="registry.redhat.io/openshift-serverless-1/kn-client-kn-rhel8"
  ["postgresql"]="registry.redhat.io/rhel9/postgresql-13"
  ["skopeo-copy"]="registry.redhat.io/rhel9/skopeo"
  ["s2i"]="registry.redhat.io/source-to-image/source-to-image-rhel8"
  ["ubi-minimal"]="registry.redhat.io/ubi9/ubi-minimal"
  ["java"]="registry.redhat.io/ubi9/openjdk-17"
)

# Find latest version/tag for an image
find_latest_version() {
  local image=$1
  # Try to get version from Labels first
  local version=$(skopeo inspect docker://${image} 2>/dev/null | jq -r '.Labels.version // empty')

  # If no version label, get latest tag
  if [[ -z "$version" ]]; then
    version=$(skopeo list-tags docker://${image} | jq -r '.Tags[]' | sort -r | grep -v '\-[a-z0-9\.]*$' | head -n 1)
  fi

  echo "$version"
}

# Get SHA digest for an image:tag
get_image_sha() {
  local image_url=$1
  skopeo inspect --raw docker://${image_url} | jq -r '.manifests[0].digest // .digest'
}

# Update image SHA in YAML files
update_yaml_files() {
  local image_prefix=$1
  local image_sha=$2

  echo "Updating: ${image_prefix} -> ${image_sha}"

  # Update all YAML files
  sed -i -E "s%(${image_prefix}).*%\1@${image_sha}%" config/openshift/base/operator.yaml
  sed -i -E "s%(${image_prefix}).*%\1@${image_sha}%" operatorhub/openshift/config.yaml
  sed -i -E "s%(${image_prefix}).*%\1@${image_sha}%" operatorhub/openshift/release-artifacts/bundle/manifests/*.yaml
  find cmd/openshift/operator/kodata/tekton-addon/addons/ -type f -name "*.yaml" -exec sed -i -E "s%(${image_prefix}).*%\1@${image_sha}%" {} +
}

# Main
main() {
  echo "Updating Red Hat images to latest SHAs..."
  echo

  for image_name in "${!IMAGES[@]}"; do
    image_registry="${IMAGES[$image_name]}"

    echo "Processing: $image_name"
    latest_version=$(find_latest_version "$image_registry")
    echo "  Latest version: $latest_version"

    image_url="${image_registry}:${latest_version}"
    image_sha=$(get_image_sha "$image_url")
    echo "  SHA: $image_sha"

    update_yaml_files "$image_registry" "$image_sha"
    echo
  done

  echo "✓ All images updated successfully"
}

main "$@"
