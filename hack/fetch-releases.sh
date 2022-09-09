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

    if [[ ${version} == "stable" ||  ${version} == "nightly" ]]; then
      url="https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/${version}/release.yaml"
    else
      url="https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-${version}/release.yaml"
    fi

    dest=${dirPath}/${fileName}.yaml
    http_response=$(curl -s -o ${dest} -w "%{http_code}" ${url})
    echo url: ${url}

    if [[ $http_response != "200" ]]; then
        echo "Error: failed to get $comp yaml, status code: $http_response"
        exit 1
    fi

    echo "Info: Added $comp/$fileName:$version release yaml !!"
    echo ""

    runtime=( go java nodejs python )
    for run in "${runtime[@]}"
    do
      echo "fetching PipelineRun template for runtime: $run"

      source="https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/${version}/pkg/cmd/tknpac/generate/templates/${run}.yaml"
      dest_dir="${ko_data}/tekton-addon/pipelines-as-code-templates"
      mkdir -p ${dest_dir} || true
      destination="${dest_dir}/${run}.yaml"

      http_response=$(curl -s -o ${destination} -w "%{http_code}" ${source})
      echo url: ${source}

      if [[ $http_response != "200" ]]; then
        echo "Error: failed to get pipelinerun template for $run, status code: $http_response"
        exit 1
      fi

    done
    echo ""
}


release_yaml_hub() {
  echo fetching '|' component: ${1} '|' version: ${2}
  local version=$2

  ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
  rm -rf ${ko_data}/tekton-hub
  if [ ${version} == "latest" ]
  then
    version=$(curl -sL https://api.github.com/repos/tektoncd/hub/releases | jq -r ".[].tag_name" | sort -Vr | head -n1)
    dirPath=${ko_data}/tekton-hub/0.0.0-latest
  else
    dirPath=${ko_data}/tekton-hub/${version}
  fi
  rm -rf ${dirPath} || true
  mkdir -p ${dirPath} || true

  url=""
  components="db db-migration api ui"

  for component in ${components}; do
    dest=${dirPath}/${component}
    rm -rf ${dest} || true
    mkdir -p ${dest} || true

    fileName=${component}.yaml

    [[ ${component} == "api" ]] || [[ ${component} == "ui" ]] && fileName=${component}-${TARGET}.yaml

    url="https://github.com/tektoncd/hub/releases/download/${version}/${fileName}"
    echo $url
    http_response=$(curl -s -L -o ${dest}/${fileName} -w "%{http_code}" ${url})
    echo url: ${url}
    if [[ $http_response != "200" ]]; then
      echo "Error: failed to get ${component} yaml, status code: $http_response"
      exit 1
    fi
    echo "Info: Added Hub/$fileName:$version release yaml !!"
    echo ""
  done
}

fetch_openshift_addon_tasks() {
  fetch_addon_task_script="${SCRIPT_DIR}/hack/openshift"
  local dest_dir="cmd/openshift/operator/kodata/tekton-addon/addons/02-clustertasks/source_external"
  ${fetch_addon_task_script}/fetch-tektoncd-catalog-tasks.sh ${dest_dir}
}

#Args: <target-platform> <pipelines version> <triggers version> <dashboard version> <results version> <pac version> <hub version> <chain version>
main() {
  OPERATORTOOL=$1
  TARGET=$2
  CONFIG=${3:=components.yaml}
  p_version=$(${OPERATORTOOL} -config ${CONFIG} component-version pipeline)
  t_version=$(${OPERATORTOOL} -config ${CONFIG} component-version triggers)
  c_version=$(${OPERATORTOOL} -config ${CONFIG} component-version chains)

  # get release YAML for Pipelines
  release_yaml pipeline release 00-pipelines ${p_version}

  # get release YAML for Triggers
  release_yaml triggers release 00-triggers ${t_version}
  release_yaml triggers interceptors 01-interceptors ${t_version}

  # get release YAML for Chains
  release_yaml chains release 00-chains ${c_version}

  if [[ ${TARGET} != "openshift" ]]; then
    d_version=$(${OPERATORTOOL} -config ${CONFIG} component-version dashboard)
    # get release YAML for Dashboard
    release_yaml dashboard tekton-dashboard-release 00-dashboard ${d_version}
    release_yaml dashboard tekton-dashboard-release-readonly 00-dashboard ${d_version}

    r_version=$(${OPERATORTOOL} -config ${CONFIG} component-version results)
    # get release YAML for Results
    release_yaml results release 00-results ${r_version}
  else
    pac_version=$(${OPERATORTOOL} -config ${CONFIG} component-version pipelines-as-code)
    release_yaml_pac pipelinesascode release ${pac_version}
    fetch_openshift_addon_tasks
  fi

  hub_version=$(${OPERATORTOOL} -config ${CONFIG} component-version hub)
  release_yaml_hub hub ${hub_version}
  
  echo updated payload tree
  find cmd/${TARGET}/operator/kodata
}

main $@

