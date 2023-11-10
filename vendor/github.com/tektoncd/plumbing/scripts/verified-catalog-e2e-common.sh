#!/usr/bin/env bash

# Copyright 2023 The Tekton Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Helper functions for E2E tests.
source $(dirname ${BASH_SOURCE})/e2e-tests.sh

# Define a custom kubectl path if you like
KUBECTL_CMD=${KUBECTL_CMD:-kubectl}

# Dependency checks
## Bash must be 4 or greater to support associative arrays
if [ "${BASH_VERSINFO:-0}" -lt 4 ];then
    echo "this script must be executed in bash >= 4"
    exit 1
fi

## Commands
function require_command() {
    if ! command -v ${1} &> /dev/null;then
        echo "required command '${1}' not be found"
        exit 1
    fi
}

require_command ${KUBECTL_CMD} python3

function test_yaml_can_install() {
    # Validate that all the Task CRDs in this repo are valid by creating them in a NS.
    readonly ns="task-ns"
    all_tasks="$*"
    ${KUBECTL_CMD} create ns "${ns}" || true
    local runtest
    for runtest in ${all_tasks}; do
        # remove task/ from beginning
        local runtestdir=${runtest#*/}
        # remove /tests from end
        local testname=${runtestdir%%/*}
        runtest=${runtest//tests}

        # in case a task is being removed then it's directory
        # doesn't exists, so skip the test for YAML
        [ ! -d "${runtest%%/*}/${testname}" ] && continue

        input="${runtest}${testname}.yaml"

        echo "Checking ${testname}"
        ${KUBECTL_CMD} -n ${ns} apply -f <(sed "s/namespace:.*/namespace: task-ns/" "${input}")
    done

    ${KUBECTL_CMD} delete ns ${ns} >/dev/null || true
}

function create_task() {
    local tns=${1}
    local taskdir=${2}

    # In case of rerun it's fine to ignore this error
    ${KUBECTL_CMD} create namespace ${tns} >/dev/null 2>/dev/null || :

    # Install the task itself first. We can only have one YAML file
    yaml=$(printf  ${taskdir}/*.yaml)
    started=$(date '+%Hh%M:%S')
    echo "${started} STARTING: ${testname}/${version} "
    # dry-run this YAML to validate and also get formatting side-effects.
    ${KUBECTL_CMD} -n ${tns} create -f ${yaml} --dry-run=client -o yaml >${TMPF}

    # Make sure we have deleted the content, this is in case of rerun
    # and namespace hasn't been cleaned up or there is some Cluster*
    # stuff, which really should not be allowed.
    ${KUBECTL_CMD} -n ${tns} delete -f ${TMPF} >/dev/null 2>/dev/null || true
    ${KUBECTL_CMD} -n ${tns} create -f ${TMPF}

    # Install resource and run
    for yaml in ${runtest}/*.yaml;do
        cp ${yaml} ${TMPF}

        # Make sure we have deleted the content, this is in case of rerun
        # and namespace hasn't been cleaned up or there is some Cluster*
        # stuff, which really should not be allowed.
        ${KUBECTL_CMD} -n ${tns} delete -f ${TMPF} >/dev/null 2>/dev/null || true
        ${KUBECTL_CMD} -n ${tns} create -f ${TMPF}
    done
}

# Check whether test folder exists or not inside task dir
# if not then run the tests for next task (if any)
function check_test_folder_exist() {
    local runtest=${1}
    local taskdir=${runtest%/*}
    local skipit=
    [ ! -d $runtest ] && false

    ls ${taskdir}/*.yaml 2>/dev/null >/dev/null || false

    true
}

function test_task_creation() {
    local runtest
    declare -A task_to_wait_for

    for runtest in $@;do
        # remove task/ from beginning
        local runtestdir=${runtest#*/}
        # remove /0.1/tests from end
        local testname=${runtestdir%%/*}
        # remove /tests from end
        local taskdir=${runtest%/*}
        local version=$(basename $(basename $(dirname $runtest)))
        local tns="${testname}-${version}"

        # check whether test folder exists or not inside task dir
        # if not then run the tests for next task (if any)
        check_test_folder_exist ${runtest} || continue

        create_task "${tns}" "${taskdir}"

        task_to_wait_for["$testname/${version}"]="${tns}|$started" 
    done

    # I would refactor this to a function but bash limitation is too great, really need a rewrite the sooner
    # the uglness to pass a hashmap to a function https://stackoverflow.com/a/17557904/145125
    local cnt=0
    local all_status=''
    local reason=''
    local maxloop=60 # 10 minutes max

    set +x
    while true;do
        # If we have timed out then show failures of what's remaining in
        # task_to_wait_for we assume only first one fails this
        [[ ${cnt} == "${maxloop}" ]] && {
            for testname in "${!task_to_wait_for[@]}";do
                target_ns=${task_to_wait_for[$testname]}
                show_failure "${testname}" "${target_ns}"
            done
        }
        [[ -z ${task_to_wait_for[*]} ]] && {
            break
        }

        for testname in "${!task_to_wait_for[@]}";do
            target_ns=${task_to_wait_for[$testname]%|*}
            started=${task_to_wait_for[$testname]#*|}
            # sometimes we don't get all_status and reason in one go so
            # wait until we get the reason and all_status for 5 iterations
            for tektontype in pipelinerun taskrun;do
                for _ in {1..10}; do
                    all_status=$(${KUBECTL_CMD} get -n ${target_ns} ${tektontype} --output=jsonpath='{.items[*].status.conditions[*].status}')
                    reason=$(${KUBECTL_CMD} get -n ${target_ns} ${tektontype} --output=jsonpath='{.items[*].status.conditions[*].reason}')
                    [[ ! -z ${all_status} ]] && [[ ! -z ${reason} ]] && break
                    sleep 1
                done
                # No need to check taskrun if pipelinerun has been set
                [[ ! -z ${all_status} ]] && [[ ! -z ${reason} ]] && break
            done

            if [[ -z ${all_status} || -z ${reason} ]];then
                echo "Could not find a created taskrun or pipelinerun in ${target_ns}"
            fi

            breakit=True
            for status in ${all_status};do
                [[ ${status} == *ERROR || ${reason} == *Fail* || ${reason} == Couldnt* ]] && show_failure ${testname} ${target_ns}
                if [[ ${status} != True ]];then
                    breakit=
                fi
            done

            if [[ ${breakit} == True ]];then
                unset task_to_wait_for[$testname]
                [[ -z ${CATALOG_TEST_SKIP_CLEANUP} ]] && ${KUBECTL_CMD} delete ns ${target_ns} >/dev/null
                echo "${started}::$(date '+%Hh%M:%S') SUCCESS: ${testname} testrun has successfully executed" ;
            fi
        done

        sleep 10
        cnt=$((cnt+1))
    done
    set -x 
}

function show_failure() {
    local testname=$1 tns=$2

    echo "FAILED: ${testname} task has failed to comeback properly" ;
    echo "--- Task Dump"
    ${KUBECTL_CMD} get -n ${tns} task -o yaml
    echo "--- Pipeline Dump"
    ${KUBECTL_CMD} get -n ${tns} pipeline -o yaml
    echo "--- PipelineRun Dump"
    ${KUBECTL_CMD} get -n ${tns} pipelinerun -o yaml
    echo "--- TaskRun Dump"
    ${KUBECTL_CMD} get -n ${tns} taskrun -o yaml
    echo "--- Container Logs"
    for pod in $(${KUBECTL_CMD} get pod -o name -n ${tns}); do
        ${KUBECTL_CMD} logs --all-containers -n ${tns} ${pod} || true
    done
    exit 1
}

function install_pipeline_crd() {
  local latestreleaseyaml
  echo ">> Deploying Tekton Pipelines"
  if [[ -n ${RELEASE_YAML} ]];then
	latestreleaseyaml=${RELEASE_YAML}
  else
    latestreleaseyaml="https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml"
  fi
  [[ -z ${latestreleaseyaml} ]] && fail_test "Could not get latest released release.yaml"
  ${KUBECTL_CMD} apply -f ${latestreleaseyaml} ||
      fail_test "Build pipeline installation failed"

  # Make sure that eveything is cleaned up in the current namespace.
  for res in pipelineresources tasks pipelines taskruns pipelineruns; do
    ${KUBECTL_CMD} delete --ignore-not-found=true ${res}.tekton.dev --all
  done

  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running tekton-pipelines || fail_test "Tekton Pipeline did not come up"
}

function test_tasks {
    local cnt=0
    local task_to_tests=""

    for runtest in $@;do
        task_to_tests="${task_to_tests} ${runtest}"
        if [[ ${cnt} == "${MAX_NUMBERS_OF_PARALLEL_TASKS}" ]];then
            test_task_creation "${task_to_tests}"
            cnt=0
            task_to_tests=""
            continue
        fi
        cnt=$((cnt+1))
    done

    # in case if there are some remaining tasks
    if [[ -n ${task_to_tests} ]];then
        test_task_creation "${task_to_tests}"
    fi
}

function convert_directory_structure() {
    # Copy the resources to a temp directory to futher process. 
    # Temp directory structure: ${TMPD}/${resource}/${version} (e.g. /tmp/xxx/task/golang-build/dev)
    if [[ ! -z ${TEST_RUN_NIGHTLY_TESTS} ]];then
        # Iterate through all release when testing all releases in nightly test, copy content in each release (version)
        # to a flatterned temp directory
        git fetch --tags --force
        cur_branch=$(git rev-parse --abbrev-ref HEAD)

        for version_tag in $(git tag) 
        do
            git checkout "tags/${version_tag}"

            version="$( echo $version_tag | tr '.' '-' )"
            resources=$(ls -d task/*)

            for resource in ${resources};do
                cp_dir=${TMPD}/${resource}/${version}
                mkdir -p ${cp_dir}
                cp -r ./${resource}/* ${cp_dir}
            done
        done
        git checkout ${cur_branch}
    else 
        # If the test is triggered as PR merge check, just test the content in the branch head
        version="dev"
        resources=$(ls -d task/*)

        for resource in ${resources};do
            cp_dir=${TMPD}/${resource}/${version}
            mkdir -p ${cp_dir}
            cp -r ./${resource}/* ${cp_dir}
        done
    fi
}

function echo_local_test_helper_info {
    cat <<EOF
This script will run a single task to help developers testing directly a
single task without sending it to CI.
You need to specify the task name as the argument. For example :
${0} golang-build
will run the tests for golang-build. You can add --nightly flag to do a local nightly test.
EOF
}
