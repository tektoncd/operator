#!/usr/bin/env bash

set -u -e

# Ensure GOTOOLCHAIN is set to auto to allow Go 1.25+ to be downloaded
export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"

declare -r SCRIPT_DIR=$(cd $(dirname "$0")/.. && pwd)
TARGET=""
FORCE_FETCH_RELEASE=""

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
      if [[ ${releaseFileName} == "release-full" ]]; then
        dir="dashboard/tekton-dashboard-fullaccess"
      fi

      if [[ ${releaseFileName} == "release" ]]; then
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
    dirPath=${comp_dir}/${dirVersion}

    # destination file
    dest=${dirPath}/${destFileName}.yaml

    if [ -f "$dest" ] && [ $FORCE_FETCH_RELEASE = "false" ]; then
      label="app.kubernetes.io/version: \"$version\""
      label2="app.kubernetes.io/version: $version"
      label3="version: \"$version\""
      if grep -Eq "$label" $dest || grep -Eq "$label2" $dest || grep -Eq "$label3" $dest;
      then
          echo "release file already exist with required version, skipping!"
          echo ""
          return
      fi
    fi

    # before adding releases, remove existing version directories
    # ignore while adding for interceptors
    if [[ ${releaseFileName} != "interceptors" ]] ; then
      rm -rf ${comp_dir}/*
    fi

    # create a directory
    mkdir -p ${dirPath} || true

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

    # Add OpenShift specific files for triggers
    if [[ ${TARGET} == "openshift" ]] && [[ ${comp} == "triggers" ]]; then
      cp ${ko_data}/openshift/triggers-rbac/* ${dirPath}/
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


# Function to install yq if not available
install_yq() {
  if ! command -v yq &> /dev/null; then
    echo "yq not found, installing..."
    curl -L https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -o /usr/local/bin/yq
    chmod +x /usr/local/bin/yq
    echo "yq installed successfully"
  else
    echo "yq is already available"
  fi
}

# release_yaml_github <component>
release_yaml_github() {
  local github_component version releaseFileName destFileName component url

  component=$1
  echo fetching $component release yaml from github

  # Install yq if not available
  install_yq

  github_component=$(yq .$component.github ${CONFIG})
  version=$(yq .$component.version ${CONFIG})
  releaseFileName=release-$version.yaml
  destFileName=$releaseFileName

  echo "$github_component version is $version"
  case $version in
    latest)
      dirVersion=$(curl -sL https://api.github.com/repos/$github_component/releases | jq -r ".[].tag_name" | sort -Vr | head -n1)
      ;;
    *)
      dirVersion=${version/v/}
      ;;
  esac
  url="https://github.com/$github_component/releases/download/${version}/${releaseFileName}"
  echo "URL to download Release YAML is : $url"

    ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
    comp_dir=${ko_data}/${component}
    dirPath=${comp_dir}/${dirVersion}

    # destination file
    dest=${dirPath}/${destFileName}
    echo $dest

    if [ -f "$dest" ] && [ $FORCE_FETCH_RELEASE = "false" ]; then
      label="app.kubernetes.io/version: \"$version\""
      label2="app.kubernetes.io/version: $version"
      label3="version: \"$version\""
      if grep -Eq "$label" $dest || grep -Eq "$label2" $dest || grep -Eq "$label3" $dest;
      then
          echo "release file already exist with required version, skipping!"
          echo ""
          return
      fi
    fi

    # create a directory
    mkdir -p ${dirPath} || true

    http_response=$(curl -s -L -o ${dest} -w "%{http_code}" ${url})
    if [[ $http_response != "200" ]]; then
        echo "Error: failed to get $component yaml, status code: $http_response"
        exit 1
    fi
    echo "Info: Added $component/$releaseFileName:$version release yaml !!"

}

# release_yaml_pac <component> <release-yaml-name> <version>
release_yaml_pac() {
    echo fetching '|' component: ${1} '|' file: ${2} '|' version: ${3}
    local comp=$1
    local fileName=$2
    local version=$3

    ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
    dirPath=${ko_data}/tekton-addon/pipelines-as-code/${version}

    if [[ ${version} == "stable" ||  ${version} == "nightly" ]]; then
      url="https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/${version}/release.yaml"
    else
      url="https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-${version}/release.yaml"
    fi

    dest=${dirPath}/${fileName}.yaml

     if [ -f "$dest" ] && [ $FORCE_FETCH_RELEASE = "false" ]; then
        label="app.kubernetes.io/version: \"$version\""
        if grep -Eq "$label" $dest;
          then
            echo "release file already exist with required version, skipping!"
            echo ""
            return
        fi
     else
         rm -rf ${dirPath} || true
         mkdir -p ${dirPath} || true

         http_response=$(curl -s -o ${dest} -w "%{http_code}" ${url})
         echo url: ${url}

         if [[ $http_response != "200" ]]; then
             echo "Error: failed to get $comp yaml, status code: $http_response"
             exit 1
         fi

         echo "Info: Added $comp/$fileName:$version release yaml !!"
         echo ""
     fi

    runtime=( go java nodejs python generic )
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

release_yaml_manualapprovalgate() {
  echo fetching '|' component: manual-approval-gate '|' version: ${1}
  local version=$1

  ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
  if [ ${version} == "latest" ]
  then
    version=$(curl -sL https://api.github.com/repos/openshift-pipelines/manual-approval-gate/releases | jq -r ".[].tag_name" | sort -Vr | head -n1)
    dirPath=${ko_data}/manual-approval-gate/0.0.0-latest
  else
    dirVersion=${version//v}
    dirPath=${ko_data}/manual-approval-gate/${dirVersion}
  fi
  mkdir -p ${dirPath} || true

  dest=${dirPath}
  fileName=release-${TARGET}.yaml
  destinationFile=${dest}/${fileName}

  if [ -f "$destinationFile" ] && [ $FORCE_FETCH_RELEASE = "false" ]; then
        if grep -Eq "$version" $destinationFile;
        then
            echo "release file already exist with required version, skipping!"
            echo ""
        fi
  fi

  url="https://github.com/openshift-pipelines/manual-approval-gate/releases/download/${version}/${fileName}"
  echo ${url}
  http_response=$(curl -s -L -o ${destinationFile} -w "%{http_code}" ${url})
  echo url: ${url}
  if [[ $http_response != "200" ]]; then
    echo "Error: failed to get manual-approval-gate yaml, status code: $http_response"
    exit 1
  fi
  echo "Info: Added Manual-Approval-Gate/$fileName:$version release yaml !!"
  echo ""

}


release_yaml_hub() {
  echo fetching '|' component: ${1} '|' version: ${2}
  local version=$2

  ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
  if [ ${version} == "latest" ]
  then
    version=$(curl -sL https://api.github.com/repos/tektoncd/hub/releases | jq -r ".[].tag_name" | sort -Vr | head -n1)
    dirPath=${ko_data}/tekton-hub/0.0.0-latest
  else
    dirPath=${ko_data}/tekton-hub/${version}
  fi
  mkdir -p ${dirPath} || true

  url=""
  components="db db-migration api ui hub-info"

  for component in ${components}; do
    echo fetching Hub '|' component: ${component} '|' version: ${2}

    dest=${dirPath}/${component}
    fileName=${component}.yaml
    destinationFile=${dest}/${fileName}

    if [ -f "$destinationFile" ] && [ $FORCE_FETCH_RELEASE = "false" ]; then
          if grep -Eq "$version" $destinationFile;
          then
              echo "release file already exist with required version, skipping!"
              echo ""
              continue
          fi
    fi

    rm -rf ${dest} || true
    mkdir -p ${dest} || true

    [[ ${component} == "api" ]] || [[ ${component} == "ui" ]] && fileName=${component}-${TARGET}.yaml

    url="https://github.com/tektoncd/hub/releases/download/${version}/${fileName}"
    echo $url
    http_response=$(curl -s -L -o ${destinationFile} -w "%{http_code}" ${url})
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
  local dest_dir='cmd/openshift/operator/kodata/tekton-addon/addons/06-ecosystem/tasks'
  ${fetch_addon_task_script}/fetch-tektoncd-catalog-tasks.sh ${dest_dir} "ecosystem_tasks"
  dest_dir='cmd/openshift/operator/kodata/tekton-addon/addons/06-ecosystem/stepactions'
  ${fetch_addon_task_script}/fetch-tektoncd-catalog-tasks.sh ${dest_dir} "ecosystem_stepactions"
}

copy_pruner_yaml() {
  srcPath=${SCRIPT_DIR}/config/pruner
  ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
  dstPath=${ko_data}/tekton-pruner
  rm -rf $dstPath
  cp -r $srcPath $dstPath
}

main() {
  TARGET=$1
  CONFIG=${2:=components.yaml}
  FORCE_FETCH_RELEASE=$3
  p_version=$(go run ./cmd/tool component-version ${CONFIG} pipeline)
  t_version=$(go run ./cmd/tool component-version ${CONFIG} triggers)
  c_version=$(go run ./cmd/tool component-version ${CONFIG} chains)
  r_version=$(go run ./cmd/tool component-version ${CONFIG} results)

  # get release YAML for Pipelines
  release_yaml pipeline release 00-pipelines ${p_version}

  # get release YAML for Triggers
  release_yaml triggers release 00-triggers ${t_version}
  release_yaml triggers interceptors 01-interceptors ${t_version}

  # get release YAML for Chains
  release_yaml chains release 00-chains ${c_version}

  # get release YAML for Results
  if [[ ${TARGET} == "openshift" ]]
  then
      release_yaml results release_base 00-results ${r_version}
  else
      release_yaml results release 00-results ${r_version}
  fi

  if [[ ${TARGET} != "openshift" ]]; then
    d_version=$(go run ./cmd/tool component-version ${CONFIG} dashboard)
    # get release YAML for Dashboard
    release_yaml dashboard release-full 00-dashboard ${d_version}
    release_yaml dashboard release 00-dashboard ${d_version}
  else
    pac_version=$(go run ./cmd/tool component-version ${CONFIG} pipelines-as-code)
    release_yaml_pac pipelinesascode release ${pac_version}
    fetch_openshift_addon_tasks
  fi

  hub_version=$(go run ./cmd/tool component-version ${CONFIG} hub)
  release_yaml_hub hub ${hub_version}

  mag_version=$(go run ./cmd/tool component-version ${CONFIG} manual-approval-gate)
  release_yaml_manualapprovalgate ${mag_version}

  # copy pruner rbac/sa yaml
  copy_pruner_yaml
  release_yaml_github pruner

  echo updated payload tree
  find cmd/${TARGET}/operator/kodata
}

main $@
