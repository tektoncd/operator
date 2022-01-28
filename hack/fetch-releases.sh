#!/usr/bin/env bash

set -u -e

declare -r SCRIPT_DIR=$(cd $(dirname "$0")/.. && pwd)
TARGET=""

# release_yaml <component> <release-yaml-name> <target-file-name> <version>
release_yaml() {
  echo fetching '|' component: ${1} '|' file: ${3} '|' version: ${4}
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

    if [[ $comp == "dashboard" ]]; then
      if [[ ${releaseFileName} == "tekton-dashboard-release" ]]; then
        dir="dashboard/tekton-dashboard-fullaccess"
      fi

      if [[ ${releaseFileName} == "tekton-dashboard-release-readonly" ]]; then
        dir="dashboard/tekton-dashboard-readonly"
      fi
    fi


    #nightly -> 0.0.0-nightly`
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
    echo url: ${url}

    if [[ $http_response != "200" ]]; then
        echo "Error: failed to get $comp yaml, status code: $http_response"
        exit 1
    fi

    # Add OpenShift specific files for pipelines
    if [[ ${TARGET} == "openshift" ]] && [[ ${comp} == "pipeline" ]]; then
      cp -r ${ko_data}/openshift/00-prereconcile ${comp_dir}/
      cp ${ko_data}/openshift/pipelines-rbac/* ${dirPath}/
    fi

    if [[ ${comp} == "dashboard" ]]; then
      sed -i '/aggregationRule/,+3d' ${dest}
    fi

    echo "Info: Added $comp/$releaseFileName:$version release yaml !!"
    if [[ ${comp} == "results" ]]; then
      grep 'version' ${dest} | head -n 1 | tr -d ' ' || true
    else
      grep 'app.kubernetes.io/version' ${dest} | head -n 1 | sed 's/[[:space:]]*app.kubernetes.io\///' || true
    fi
    echo ""
}

# release_yaml_pac <component> <release-yaml-name> <version>
release_yaml_pac() {
    echo fetching '|' component: ${1} '|' file: ${2} '|' version: ${3}
    local comp=$1
    local fileName=$2
    local version=$3

    ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
    dirPath=${ko_data}/tekton-addon/pipelines-as-code/${version}
    rm -rf ${dirPath} || true
    mkdir -p ${dirPath} || true

    url="https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-${version}/release-${version}.yaml"

    dest=${dirPath}/${fileName}.yaml
    http_response=$(curl -s -o ${dest} -w "%{http_code}" ${url})
    echo url: ${url}

    if [[ $http_response != "200" ]]; then
        echo "Error: failed to get $comp yaml, status code: $http_response"
        exit 1
    fi

    echo "Info: Added $comp/$fileName:$version release yaml !!"
    echo ""
}

#Args: <target-platform> <pipelines version> <triggers version> <dashboard version> <results version> <pac version>
main() {
  TARGET=$1
  p_version=${2}
  t_version=${3}


  release_yaml pipeline release 00-pipelines ${p_version}

  release_yaml triggers release 00-triggers ${t_version}
  release_yaml triggers interceptors 01-interceptors ${t_version}

  if [[ ${TARGET} != "openshift" ]]; then
    d_version=${4}
    release_yaml dashboard tekton-dashboard-release 00-dashboard ${d_version}
    release_yaml dashboard tekton-dashboard-release-readonly 00-dashboard ${d_version}

    r_version=${5}
    release_yaml results release 00-results ${r_version}
  else
    pac_version=${6}
    release_yaml_pac pipelinesascode release ${pac_version}
  fi
}

main $@

echo updated payload tree
find cmd/${1}/operator/kodata
