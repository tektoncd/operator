#!/usr/bin/env bash

set -u

function set_version_label() {
  # Set old_version correctly, adding "v" prefix unless it's "devel"
  local old_version
  if [[ "${1}" == "devel" ]]; then
    old_version="devel"
  else
    old_version="v${1}"
  fi
  shift

  local operator_version="v${1}"; shift
  local operator_platforms=${*:-"kubernetes openshift"}

  echo ${operator_platforms}
  printf "%-20s: %s\n" "Old version" "${old_version}"
  printf "%-20s: %s\n" "New version" "${operator_version}"
  printf "%-20s: %s\n" "Platforms" "${operator_platforms}"
  echo '---------------------'

  for platform in ${operator_platforms}; do
    echo updating version labels for platform: ${platform}, from version: ${old_version} to version: ${operator_version}

    # Ensure version replacement removes quotes if present
    sed -i -E \
      -e "s/(operator.tekton.dev\/release: )\"?${old_version}\"?/\1${operator_version}/g" \
      -e "s/(app.kubernetes.io\/version: )\"?${old_version}\"?/\1${operator_version}/g" \
      -e "s/(version: )\"?${old_version}\"?/\1${operator_version}/g" \
      -e "s/(\"-version\"), \"?${old_version}\"?/\1, ${operator_version}/g" config/base/*.yaml

    sed -i -E \
      -e "s/(operator.tekton.dev\/release: )\"?${old_version}\"?/\1${operator_version}/g" \
      -e "s/(app.kubernetes.io\/version: )\"?${old_version}\"?/\1${operator_version}/g" \
      -e "s/(version: )\"?${old_version}\"?/\1${operator_version}/g" \
      -e "s/(\"-version\"), \"?${old_version}\"?/\1, ${operator_version}/g" config/${platform}/base/*.yaml

    sed -i -E "s/(value: )\"?${old_version}\"?/\1${operator_version}/g" config/${platform}/base/operator.yaml
  done

  # Update cabundles only for openshift
  sed -i -E \
    -e "s/(operator.tekton.dev\/release: )\"?${old_version}\"?/\1${operator_version}/g" \
    -e "s/(app.kubernetes.io\/version: )\"?${old_version}\"?/\1${operator_version}/g" \
    -e "s/(version: )\"?${old_version}\"?/\1${operator_version}/g" \
    -e "s/(\"-version\"), \"?${old_version}\"?/\1, ${operator_version}/g" cmd/openshift/operator/kodata/cabundles/*.yaml
}



function commit_changes() {
  release_version=${1}; shift

  git add -u cmd/
  git add -u config/
  git commit -m "Freezing Component versions and updating version labels"
}

function checkout_release_branch() {
  local remote_name=${1}
  shift
  local release_version=${1}

  local branch_name=release-v$(formated_majorminorx ${release_version})

  base_branch=${remote_name}/main

  echo checking if branch \"${remote_name}/${branch_name}\"...
  result=$(git ls-remote ${remote_name} ${branch_name} | wc -l | tr -d " ")
  if [[ ${result} == 1 ]]; then
    echo remote branch \"${remote_name}/${branch_name}\" exists
    base_branch=${remote_name}/${branch_name}
  else
    echo remote branch \"${remote_name}/${branch_name}\" does not exist
  fi

  echo checking out $base_branch as ${branch_name}
  git checkout -B ${branch_name} ${base_branch}
}

function push_release_branch() {
  local remote_name=${1}; shift
  local release_version=${1}
  local branch_name=release-v$(formated_majorminorx ${release_version})

  until false; do
    read -p "push ${branch_name} to ${remote_name}/ (Y/n): " confirmation
    case ${confirmation} in
      y|Y|yes|YES|Yes|'')
        echo "pushing ${branch_name} to ${remote_name}"
        git push ${remote_name} ${branch_name}:${branch_name}
        break
        ;;
      n|N|no|NO|no)
        echo "aborting pushing ${branch_name} to ${remote_name}"
        return 2
        ;;
      *)
        echo unknown input. type y/Y/n/N
        ;;
    esac
  done
}

function formated_majorminorx() {
  local version=${1}
  majorminorx=$(echo ${version} | sed  's/\([0-9]*\.[0-9]*\.\).*/\1x/')
  echo $majorminorx
}
