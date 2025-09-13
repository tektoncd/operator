#!/usr/bin/env bash
set -e -u -o pipefail

declare -r SCRIPT_NAME=$(basename "$0")
declare -r SCRIPT_DIR=$(cd $(dirname "$0") && pwd)
declare -r USERNAME=${REGISTRY_USER}
declare -r PASSWORD=${REGISTRY_PASSWORD}

log() {
    local level=$1; shift
    echo -e "$level: $@"
}


err() {
    log "ERROR" "$@" >&2
}

info() {
    log "INFO" "$@"
}

die() {
    local code=$1; shift
    local msg="$@"; shift
    err $msg
    exit $code
}

usage() {
  local msg="$1"
  cat <<-EOF
Error: $msg

USAGE:
    REGISTRY_USER=<registry user name> REGISTRY_PASSWORD=<registry password> $SCRIPT_NAME

Example:
  REGISTRY_USER=johnsmith REGISTRY_PASSWORD=pass123 $SCRIPT_NAME
EOF
  exit 1
}

#declare -r CATALOG_VERSION="release-v0.7"

declare -A IMAGES=(
  ["buildah"]="registry.redhat.io/rhel9/buildah"
  ["kn"]="registry.redhat.io/openshift-serverless-1/kn-client-kn-rhel8"
  ["postgresql"]="registry.redhat.io/rhel9/postgresql-16"
  ["skopeo-copy"]="registry.redhat.io/rhel9/skopeo"
  ["s2i"]="registry.redhat.io/source-to-image/source-to-image-rhel9"
  ["ubi-minimal"]="registry.redhat.io/ubi9/ubi-minimal"
  ["java"]="registry.redhat.io/ubi9/openjdk-17"
)

registry_login() {
  podman login --username=${USERNAME} --password=${PASSWORD} registry.redhat.io
}

find_latest_versions() {
  local image_registry=${1:-""}
  local latest_version=""
  if ! skopeo inspect docker://${image_registry} 2>/dev/null | jq '.Labels.version' | tr -d '"'
  then
    podman search --list-tags ${image_registry}  | grep -v NAME | tr -s ' ' | cut -d ' ' -f 2  | sort -r | grep -v '\-[a-z0-9\.]*$' | head -n 1
  fi
}

find_sha_from_tag() {
  local image_url=${1:-""}
  podman run --rm docker.io/mplatform/manifest-tool:v2.0.0 --username=${USERNAME} --password=${PASSWORD}  inspect $image_url --raw | jq '.digest' | tr -d '"'
}

update_image_sha() {
  local image_prefix=${1:-""}
  shift
  local image_sha=${1:-""}
  shift
  echo replacemnet var = ${image_prefix}
  sed -i -E 's%('${image_prefix}').*%\1@'${image_sha}'%' config/openshift/base/operator.yaml
  sed -i -E 's%('${image_prefix}').*%\1@'${image_sha}'%' operatorhub/openshift/config.yaml
  sed -i -E 's%('${image_prefix}').*%\1@'${image_sha}'%' operatorhub/openshift/release-artifacts/bundle/manifests/*.yaml
  find cmd/openshift/operator/kodata/tekton-addon/addons/ -type f -name "*.yaml" -exec sed -i -E 's%('${image_prefix}').*%\1@'${image_sha}'%' {} +
}


main() {
  registry_login
  for image in ${!IMAGES[@]}; do
    latest_version=$(find_latest_versions ${IMAGES[$image]})
    echo latest_version=$latest_version
    image_url="${IMAGES[$image]}":"${latest_version}"
    echo $image_url
    image_sha=$(find_sha_from_tag "${image_url}")
    echo image_sha=${image_sha}
    update_image_sha "${IMAGES[$image]}" $image_sha

  done

  return $?
}

main "$@"
