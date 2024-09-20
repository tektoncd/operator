#!/usr/bin/env bash
set -e -u -o pipefail

declare -r SCRIPT_NAME=$(basename "$0")
declare -r SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

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
    $SCRIPT_NAME DEST_DIR

Example:
  $SCRIPT_NAME cmd/openshift/operator/kodata/tekton-addon/addons/02-clustertasks/source_external
EOF
  exit 1
}

#declare -r CATALOG_VERSION="release-v0.7"

declare -r TEKTON_CATALOG="https://raw.githubusercontent.com/tektoncd/catalog"
declare -A TEKTON_CATALOG_TASKS=(
  # Need to remove version param
  ["git-clone"]="0.9"
  ["kn"]="0.2"
  ["kn-apply"]="0.2"
  ["skopeo-copy"]="0.3"
  ["tkn"]="0.4"
  # Those tasks are managed directly in the repository
  # ["buildah"]="0.1"
  # ["openshift-client"]="0.2"
)
declare -r TEKTON_ECOSYSTEM="https://raw.githubusercontent.com/openshift-pipelines/tektoncd-catalog"
declare -A TEKTON_ECOSYSTEM_TASKS=(
  ['task-buildah']="0.4.1"
  ['task-git-cli']="0.4.1"
  ['task-git-clone']='0.4.1'
  ['task-kn-apply']='0.2.2'
  ['task-kn']='0.2.2'
  ['task-maven']="0.3.2"
  ['task-openshift-client']="0.2.2"
  ['task-s2i-dotnet']='0.4.1'
  ['task-s2i-go']='0.4.1'
  ['task-s2i-java']='0.4.1'
  ['task-s2i-nodejs']='0.4.1'
  ['task-s2i-perl']='0.4.1'
  ['task-s2i-php']='0.4.1'
  ['task-s2i-python']='0.4.1'
  ['task-s2i-ruby']='0.4.1'
  ['task-skopeo-copy']='0.4.1'
  ['task-tkn']='0.2.2'
)
declare -A TEKTON_ECOSYSTEM_STEPACTIONS=(
  ['stepaction-git-clone']='0.4.1'
)

download_task() {
  local task_path="$1"; shift
  local task_url="$1"; shift

  info "downloading ... $t from $task_url"
  # validate url
  curl --output /dev/null --silent --head --fail "$task_url" || return 1


  cat <<-EOF > "$task_path"
# auto generated by script/update-tasks.sh
# DO NOT EDIT: use the script instead
# source: $task_url
#
---
$(curl -sLf "$task_url")
EOF

 # NOTE: helps when the original and the generated need to compared
 # curl -sLf "$task_url"  -o "$task_path.orig"

}


get_tasks() {
  local dest_dir="$1"; shift || true
  local catalog="$1"; shift || true
  local catalog_version="$1"; shift || true
  local type="$1"; shift || true
  local -n resources="$1"

  info "Downloading resources from catalog $catalog to $dest_dir directory"

  for t in ${!resources[@]} ; do
    # task filenames do not follow a naming convention,
    # some are taskname.yaml while others are taskname-task.yaml
    # so, try both before failing
    local task_url=""
    if [[ "$type" == "ecosystem_tasks" ]]; then
      task_url="$catalog/$catalog_version/tasks/$t/${resources[$t]}/${t}.yaml"
    elif [[ "$type" == 'default' ]];then
      task_url="$catalog/$catalog_version/task/$t/${resources[$t]}/${t}.yaml"
    elif [[ "$type" == 'ecosystem_stepactions' ]];then
      task_url="$catalog/$catalog_version/stepactions/$t/${resources[$t]}/${t}.yaml"
    fi

    mkdir -p "$dest_dir/$t/"
    local task_path="$dest_dir/$t/$t-task.yaml"

    download_task  "$task_path" "$task_url"  ||
      die 1 "Failed to download $t"
  done
}

create_dir_or_die() {
  local dest_dir="$1"; shift
  mkdir -p "$dest_dir" || die 1 "failed to create ${dest_dir}"
  echo $dest_dir created
}

main() {
  local type=${2:-"default"}

  case "$type" in
    "default")
      dest_dir=${1:-'cmd/openshift/operator/kodata/tekton-addon/addons/02-clustertasks/source_external'}
      resources=TEKTON_CATALOG_TASKS
      branch="main"
      catalog="$TEKTON_CATALOG"
      ;;
    "ecosystem_tasks")
      dest_dir=${1:-'cmd/openshift/operator/kodata/tekton-addon/addons/07-ecosystem/tasks'}
      resources=TEKTON_ECOSYSTEM_TASKS
      branch="p"
      catalog="$TEKTON_ECOSYSTEM"
      ;;
    "ecosystem_stepactions")
      dest_dir=${1:-'cmd/openshift/operator/kodata/tekton-addon/addons/07-ecosystem/stepactions'}
      resources=TEKTON_ECOSYSTEM_STEPACTIONS
      branch="p"
      catalog="$TEKTON_ECOSYSTEM"
      ;;
    *)
      echo "Invalid type specified: $type"
      return 1
      ;;
  esac

  [[ -z "$dest_dir"  ]] && usage "missing destination directory"
  shift

  [[ ! -d "$dest_dir" ]] && create_dir_or_die "$dest_dir" || echo "$dest_dir" exists

  get_tasks "$dest_dir" $catalog $branch $type $resources
  return $?
}

main "$@"
