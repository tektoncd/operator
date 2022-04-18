#!/usr/bin/env bash

SCRIPT_DIR=$(dirname $0)
source ${SCRIPT_DIR}/release-const.sh

declare -A components=(
  ["pipeline"]="TEKTON_PIPELINE_VERSION:tektoncd:pipeline:placeholder_version"
  ["triggers"]="TEKTON_TRIGGERS_VERSION:tektoncd:triggers:placeholder_version"
  ["results"]="TEKTON_RESULTS_VERSION:tektoncd:results:placeholder_version"
  ["dashobard"]="TEKTON_DASHBOARD_VERSION:tektoncd:dashboard:placeholder_version"
  ["chains"]="TEKTON_CHAINS_VERSION:tektoncd:chains:placeholder_version"
  ["hub"]="TEKTON_HUB_VERSION:tektoncd:hub:placeholder_version"
  ["pipelines-as-code"]="PAC_VERSION:openshift-pipelines:pipelines-as-code:placeholder_version"
)

function choose_component_versions() {
    read_exiting_versions
    for component in ${!components[@]}; do
      new_version=$(choose_version_for_component ${component})
      if [[ "$?" != 0 ]]; then
        printf "%s" ${new_version}
        exit 1
      fi
      printf "component  : %s\n" ${component}
      printf "new_version: %s\n" ${new_version}
      update_version_for_component_config ${component} ${new_version}
      save_component_versions_in_config_file ${component} ${new_version}
    done

    print_component_versions
}

function save_component_versions_in_config_file() {
    local component=${1}; shift
    local version=${1}
    env_key=$(env_key_from_config $component)
    sed -i 's/\(.*'${env_key}'=\).*/\1'${version}'/' ${config_file_path}
}

function choose_version_for_component() {
  local component=${1}; shift
  org=$(org_from_config ${component})
  project=$(project_from_config ${component})
  last_5_releases="$(last_n_releases ${org} ${project} 5)"
  if [[ "$?" != 0 ]]; then
    printf "%s", ${last_5_releases}
    return 1
  fi
  prompt_message="select ${org}/${project} version: "
  version=$(echo ${last_5_releases}  | tr ' ' '\n' | fzf --reverse --prompt="${prompt_message}" -e)
  echo ${version}
}

function update_version_for_component_config() {
    local component=${1}; shift
    local version=${1}
    config_string=${components[$component]}
    new_config_string=$(echo ${config_string} | sed 's/\(.*:\).*/\1'${version}'/')
    components[${component}]=${new_config_string}
}

function last_n_releases() {
    local org=${1:-tektoncd}; shift
    local project=${1}; shift
    local n_tags=${1:-10}; shift
    repo_url=https://api.github.com/repos/${org}/${project}/releases
    headers=""
    [[ ! -z ${GITHUB_TOKEN} ]] && headers="-H \"Authorization: TOKEN ${GITHUB_TOKEN}\""

    releases=$(curl ${headers} -sL ${repo_url})
    tag_names_array_exists=$(echo ${releases} | jq 'if type=="array" then true else false end')
    if [[ ${tag_names_array_exists} == "false" ]]; then
      printf "error querying existing releases: %s" "${releases}"
      return 1
    fi
    echo ${releases} | jq '.[].tag_name' | sort -Vr | head -n ${n_tags}
}

function read_exiting_versions() {
    source ${config_file_path}
    for component in ${!components[@]}; do
      env_key=$(env_key_from_config ${component})
      substitue_existing_verion_for_compoennt ${component} ${!env_key}
    done
    print_component_versions
}

function print_component_versions() {
  echo ""
  printf "\n%s\n" "Component Versions"
  echo "----------------------------------------------------------------------"
  format_str="%-25s%-25s%-20s\n"
  printf ${format_str} "org" "component" "version"
  echo "----------------------------------------------------------------------"
  for key in ${!components[@]}; do
    local component=$(project_from_config ${key})
    local org=$(org_from_config ${key})
    local version=$(version_from_config ${key})
    printf ${format_str} ${org} ${component} ${version}
   done
   echo "----------------------------------------------------------------------"
}

function substitue_existing_verion_for_compoennt() {
    local component=${1}; shift
    local version=${1:-"unknown"}
    config_string=${components[${component}]}
    updated_string=$(sed 's/placeholder_version/'${version}'/' <(echo ${config_string}))
    components[${component}]=${updated_string}
}

function env_key_from_config() {
  component_key=${1}
  env_key=$(field_from_config_string ${component_key} 1)
  echo ${env_key}
}

function org_from_config() {
  component_key=${1}
  org=$(field_from_config_string ${component_key} 2)
  echo ${org}
}

function project_from_config() {
  component_key=${1}
  project=$(field_from_config_string ${component_key} 3)
  echo ${project}
}

function version_from_config() {
  component_key=${1}
  current_version=$(field_from_config_string ${component_key} 4)
  echo ${current_version}
}

function field_from_config_string() {
    key=${1}; shift
    n_field=${1:-1}; shift
    config_string=${components[${key}]}
    field=$(echo ${config_string} | cut -d ":" -f ${n_field})
    echo ${field}
}