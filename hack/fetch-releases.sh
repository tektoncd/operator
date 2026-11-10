#!/usr/bin/env bash

set -u -e

declare -r SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
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
        url="https://infra.tekton.dev/tekton-nightly/${comp}/latest/${releaseFileName}.yaml"
        ;;
      latest)
        dirVersion="0.0.0-latest"
        url="https://infra.tekton.dev/tekton-releases/${comp}/latest/${releaseFileName}.yaml"
        ;;
      *)
        dirVersion=${version//v}
        url="https://infra.tekton.dev/tekton-releases/${comp}/previous/${version}/${releaseFileName}.yaml"
        ;;
    esac

    ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
    # pruner uses "pruner" directory (not "tekton-pruner") to avoid conflicts
    if [[ $comp == "pruner" ]]; then
      comp_dir=${ko_data}/${dir}
    else
      comp_dir=${ko_data}/tekton-${dir}
    fi
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

    http_response=$(curl -s -L -o ${dest} -w "%{http_code}" ${url})
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
      # For Mac Systems sed -i need to be followed by an .''
      sed -i.'' '/aggregationRule/,+3d' ${dest}
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
  # syncer-service uses non-prefixed directory (similar to pruner/manual-approval-gate)
  if [[ $component == "syncer-service" ]]; then
    comp_dir=${ko_data}/${component}
  else
    comp_dir=${ko_data}/tekton-${component}
  fi
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

  #Cleanup the Directory
  rm -rf $comp_dir || true

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
    comp_dir=${ko_data}/pipelines-as-code
    dirPath=${comp_dir}/${version}

    local pac_release_yaml=release.yaml
    if [[ ${TARGET} != "openshift" ]]; then
      pac_release_yaml=release.k8s.yaml
    fi

    if [[ ${version} == "stable" ||  ${version} == "nightly" ]]; then
      url="https://raw.githubusercontent.com/tektoncd/pipelines-as-code/${version}/${pac_release_yaml}"
    else
      url="https://raw.githubusercontent.com/tektoncd/pipelines-as-code/release-${version}/${pac_release_yaml}"
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
     fi

     # Before adding releases, remove existing version directories (same pattern as release_yaml)
     rm -rf ${comp_dir}/*
     mkdir -p ${dirPath} || true

     http_response=$(curl -s -o ${dest} -w "%{http_code}" ${url})
     echo url: ${url}

     if [[ $http_response != "200" ]]; then
         echo "Error: failed to get $comp yaml, status code: $http_response"
         exit 1
     fi

     echo "Info: Added $comp/$fileName:$version release yaml !!"
     echo ""

    runtime=( go java nodejs python generic )
    for run in "${runtime[@]}"
    do
      echo "fetching PipelineRun template for runtime: $run"

      source="https://raw.githubusercontent.com/tektoncd/pipelines-as-code/${version}/pkg/cmd/tknpac/generate/templates/${run}.yaml"
      dest_dir="${ko_data}/pipelines-as-code-templates"
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

release_yaml_custom() (
  set -euo pipefail

  local component=$1
  shift
  local filenames=("$@")
  local repository version
  repository=$(yq -er ".[\"${component}\"].github" "${CONFIG}")
  version=$(yq -er ".[\"${component}\"].version" "${CONFIG}")
  local version_dir=${version#v}
  if [[ ! ${component} =~ ^[a-z0-9-]+$ || ! ${version_dir} =~ ^[[:alnum:]][[:alnum:]._-]*$ || ${version} == "null" || ${#filenames[@]} == 0 ]]; then
    echo "ERROR: invalid component, version, or empty file list for ${component}" >&2
    return 1
  fi

  local component_dir=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata/${component}
  local destination=${component_dir}/${version_dir}
  local staging
  mkdir -p "${component_dir%/*}"
  staging=$(mktemp -d "${component_dir}.fetch-XXXXXX")

  cleanup_custom_release() {
    local status=$?
    if [[ -e ${staging}/previous && ! -e ${component_dir} ]]; then
      if ! mv "${staging}/previous" "${component_dir}"; then
        echo "ERROR: unable to restore previous payload; preserved at ${staging}/previous" >&2
        return 1
      fi
    fi
    rm -rf "${staging}"
    return "${status}"
  }
  trap cleanup_custom_release EXIT
  trap 'exit 1' HUP INT TERM

  mkdir -p "${staging}/payload/${version_dir}"

  github_fetch() {
    local repository=$1 version=$2 filename=$3
    if [[ ! ${repository} =~ ^[[:alnum:]_.-]+/[[:alnum:]_.-]+$ || ! ${version} =~ ^[[:alnum:]][[:alnum:]._-]*$ || ${version} == "null" || ! ${filename} =~ ^[[:alnum:]][[:alnum:]._-]*$ ]]; then
      echo "ERROR: invalid release source for ${component}: ${repository} ${version} ${filename}" >&2
      return 1
    fi
    local url="https://github.com/${repository}/releases/download/${version}/${filename}"
    local http_response
    echo "Fetching ${url}"
    if ! http_response=$(curl --fail --silent --show-error --location --retry 3 --output "${staging}/payload/${version_dir}/${filename}" --write-out "%{http_code}" "${url}"); then
      echo "ERROR: failed to download ${url}" >&2
      return 1
    fi
    if [[ ${http_response} != "200" ]]; then
      echo "ERROR: failed to get ${filename}, status code: ${http_response}" >&2
      return 1
    fi
    # Validate the entire stream, not just the final document. Empty separator
    # documents are allowed, but every resource must have Kubernetes metadata.
    if ! yq eval-all --exit-status '[select(. != null)] | ((length > 0) and (map(select(tag != "!!map" or (.apiVersion | tag) != "!!str" or .apiVersion == "" or (.kind | tag) != "!!str" or .kind == "" or (.metadata.name | tag) != "!!str" or .metadata.name == "")) | length == 0))' "${staging}/payload/${version_dir}/${filename}" > /dev/null; then
      echo "ERROR: ${url} is not a non-empty Kubernetes manifest stream" >&2
      return 1
    fi
  }

  local filename
  for filename in "${filenames[@]}"; do
    github_fetch "${repository}" "${version}" "${filename}"
  done

  # Keep configuration parsing and downloads in this shell so errors cannot be
  # hidden by a pipeline or subshell. Platform sources retain their own pins.
  if [[ ${TARGET} == "openshift" ]]; then
    yq --exit-status ".[\"${component}\"].openshift // [] | (map(select(.github == null or .version == null or .files == null or (.files | length) == 0)) | length == 0)" "${CONFIG}" > /dev/null
    yq -r ".[\"${component}\"].openshift // [] | .[] | . as \$o | \$o.files[] | \$o.github + \" \" + \$o.version + \" \" + ." "${CONFIG}" > "${staging}/openshift-files"
    while read -r repository version filename; do
      github_fetch "${repository}" "${version}" "${filename}"
    done < "${staging}/openshift-files"
  fi

  # Replace only after every source has downloaded and validated. Retain the
  # previous component until publication succeeds so the EXIT trap can restore it.
  if [[ -e ${component_dir} ]]; then
    mv "${component_dir}" "${staging}/previous"
  fi
  mv "${staging}/payload" "${component_dir}"
  echo "INFO: Updated ${component} payload at ${destination}"
)

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

# append_openshift_results_postgres ensures db.yaml is present exactly once in the
# OpenShift results release manifest. Repeated cat >> without a document separator
# merges the StatefulSet spec into the following ConfigMap and breaks install:
#   unable to find api field in struct ConfigMap for the json field "spec"
append_openshift_results_postgres() {
  local results_file=$1
  local db_yaml=$2
  local postgres_cm_count
  local postgres_line
  local cut_line
  local tmp_file

  if [[ ! -f "${db_yaml}" ]]; then
    echo "Warning: ${db_yaml} not found, skipping OpenShift Results postgres append"
    return
  fi
  if [[ ! -f "${results_file}" ]]; then
    echo "Error: ${results_file} not found, cannot append postgres manifest"
    exit 1
  fi

  # release_base never ships POSTGRES_DB; only our appended db.yaml does.
  postgres_cm_count=$(grep -c '^  POSTGRES_DB: tekton-results$' "${results_file}" || true)

  if [[ "${postgres_cm_count}" -eq 1 ]]; then
    echo "PostgreSQL manifest already present in ${results_file}, skipping append"
    return
  fi

  if [[ "${postgres_cm_count}" -gt 1 ]]; then
    echo "Duplicated postgres manifests (${postgres_cm_count}) in ${results_file}, repairing..."
    postgres_line=$(grep -n '^  POSTGRES_DB: tekton-results$' "${results_file}" | head -1 | cut -d: -f1)
    # Appended block starts at: apiVersion / data / POSTGRES_DB (2 lines above).
    cut_line=$((postgres_line - 2))
    if [[ "${cut_line}" -lt 1 ]]; then
      echo "Error: cannot locate postgres append start in ${results_file}"
      exit 1
    fi
    # If a --- document separator sits immediately above, drop it too.
    if [[ "${cut_line}" -gt 1 ]] && [[ "$(sed -n "$((cut_line - 1))p" "${results_file}")" == "---" ]]; then
      cut_line=$((cut_line - 1))
    fi
    tmp_file=$(mktemp)
    head -n "$((cut_line - 1))" "${results_file}" > "${tmp_file}"
    mv "${tmp_file}" "${results_file}"
  fi

  echo "Appending PostgreSQL manifest to main results manifest..."
  printf '\n---\n' >> "${results_file}"
  cat "${db_yaml}" >> "${results_file}"
  echo "PostgreSQL manifest appended to: ${results_file}"
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
      # Append PostgreSQL manifest to main results manifest for OpenShift.
      # PostgreSQL is part of main reconciliation (not PreReconcile).
      ko_data=${SCRIPT_DIR}/cmd/${TARGET}/operator/kodata
      results_file=${ko_data}/tekton-results/${r_version//v}/00-results.yaml
      db_yaml=${ko_data}/static/tekton-results/internal-db/db.yaml
      append_openshift_results_postgres "${results_file}" "${db_yaml}"
  else
      release_yaml results release 00-results ${r_version}
  fi

  if [[ ${TARGET} != "openshift" ]]; then
    d_version=$(go run ./cmd/tool component-version ${CONFIG} dashboard)
    # get release YAML for Dashboard
    release_yaml dashboard release-full 00-dashboard ${d_version}
    release_yaml dashboard release 00-dashboard ${d_version}
  fi

  pac_version=$(go run ./cmd/tool component-version ${CONFIG} pipelines-as-code)
  release_yaml_pac pipelinesascode release ${pac_version}


  if [[ ${TARGET} == "openshift" ]]; then
    fetch_openshift_addon_tasks
  fi

  mag_version=$(go run ./cmd/tool component-version ${CONFIG} manual-approval-gate)
  release_yaml_manualapprovalgate ${mag_version}

  # copy pruner rbac/sa yaml
  copy_pruner_yaml
  pruner_version=$(go run ./cmd/tool component-version ${CONFIG} pruner)
  release_yaml pruner release 00-pruner ${pruner_version}

  # Tekton Kueue
  release_yaml_github kueue

  # Tekton Multicluster Proxy AAE
  release_yaml_github multicluster-proxy-aae

  # Syncer Service
  release_yaml_github syncer-service

  # Shipwright Build
  if [[ ${TARGET} == "openshift" ]]; then
    release_yaml_custom shipwright-build release.yaml
  else
    release_yaml_custom shipwright-build release.yaml sample-strategies.yaml
  fi

  # Shared Resource CSI Driver
  if [[ ${TARGET} == "openshift" ]]; then
    release_yaml_custom shared-resource release.yaml
  fi

  echo updated payload tree
  find cmd/${TARGET}/operator/kodata
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
