#!/bin/bash

set -u -e -o pipefail

declare -r SCRIPT_DIR=$(cd $(dirname "$0")/.. && pwd)
TARGET=""

# release_yaml <component> <release-yaml-name> <target-file-name> <version>
release_yaml() {
 local comp=$1
 local releaseFileName=$2
 local destFileName=$3
 local version=$4

  # directory name => tekton-<component>
  # tekton-pipeline tekton-trigger
  local dir=$1
  if [[ $comp == "triggers" ]]; then
    dir="trigger"
  fi

  #nightly -> 0.0.0-nightly
  #latest -> find version till then -> 0.0.0-latest
  #version -> directory with version

  url=""
  case $version in
    nightly)
      dirVersion="0.0.0-nightly"
      url="https://storage.googleapis.com/tekton-releases-nightly/${comp}/latest/${releaseFileName}.yaml"
      ;;
    latest)
      dirVersion="0.0.0-latest"
      url="https://storage.googleapis.com/tekton-releases/${comp}/latest/${releaseFileName}.yaml"
      ;;
    *)
      dirVersion=${version//v}
      url="https://storage.googleapis.com/tekton-releases/${comp}/previous/${version}/${releaseFileName}.yaml"
      ;;
  esac

  ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
  comp_dir=${ko_data}/tekton-${dir}

  # before adding releases, remove existing version directories
  # ignore while adding for interceptors
  if [[ ${releaseFileName} != "interceptors" ]] ; then
    rm -rf ${comp_dir}/*
  fi

  # create a directory
  dirPath=${comp_dir}/${dirVersion}
  mkdir -p ${dirPath} || true

  # destination file
  dest=${dirPath}/${destFileName}.yaml

  http_response=$(curl -s -o ${dest} -w "%{http_code}" ${url})

  if [ $http_response != "200" ]; then
      echo "Error: failed to get $comp yaml, status code: $http_response"
      exit 1
  fi

  # Add OpenShift specific files for pipelines
  if [[ ${TARGET} == "openshift" ]] && [[ ${comp} == "pipeline" ]]; then
    cp -r ${ko_data}/openshift/00-prereconcile ${comp_dir}/
    cp ${ko_data}/openshift/pipelines-rbac/* ${dirPath}/
  fi

  echo "Info: Added $comp/$releaseFileName:$version release yaml !!"
}

#Args: TARGET PIPELINES_VERSION TRIGGERS_VERSION
main() {
  TARGET=$1

  p_version=${2}
  release_yaml pipeline release 00-pipelines ${p_version}

  t_version=${3}
  release_yaml triggers release 00-triggers ${t_version}
  release_yaml triggers interceptors 01-interceptors ${t_version}
}

main $@