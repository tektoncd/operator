#!/usr/bin/env bash
set -u

function set_version_label() {
  local operator_version="v${1}"

  shift
  local operator_platforms=${*:-"kubernetes openshift"}

  echo ${operator_platforms}
  printf "%-20s: %s\n" "Platforms" "${operator_platforms}"
  printf "%-20s: %s\n" "Operator version" "${operator_version}"
  echo '---------------------'
  for platform in ${operator_platforms}; do
    echo updating version labels for platform: ${platform}, version: ${operator_version}
    sed -i -e 's/\(operator.tekton.dev\/release\): "devel"/\1: '${operator_version}'/g' -e 's/\(app.kubernetes.io\/version\): "devel"/\1: '${operator_version}'/g' -e 's/\(version\): "devel"/\1: '${operator_version}'/g' -e 's/\("-version"\), "devel"/\1, '${operator_version}'/g' config/base/*.yaml
    sed -i -e 's/\(operator.tekton.dev\/release\): "devel"/\1: '${operator_version}'/g' -e 's/\(app.kubernetes.io\/version\): "devel"/\1: '${operator_version}'/g' -e 's/\(version\): "devel"/\1: '${operator_version}'/g' -e 's/\("-version"\), "devel"/\1, '${operator_version}'/g' config/webhooks/*.yaml
    sed -i -e 's/\(operator.tekton.dev\/release\): "devel"/\1: '${operator_version}'/g' -e 's/\(app.kubernetes.io\/version\): "devel"/\1: '${operator_version}'/g' -e 's/\(version\): "devel"/\1: '${operator_version}'/g' -e 's/\("-version"\), "devel"/\1, '${operator_version}'/g' config/${platform}/base/*.yaml
    sed -i -e 's/\(operator.tekton.dev\/release\): "devel"/\1: '${operator_version}'/g' -e 's/\(app.kubernetes.io\/version\): "devel"/\1: '${operator_version}'/g' -e 's/\(version\): "devel"/\1: '${operator_version}'/g' -e 's/\("-version"\), "devel"/\1, '${operator_version}'/g' config/${platform}/overlays/default/*.yaml
    sed -i -e 's/\(operator.tekton.dev\/release\): "devel"/\1: '${operator_version}'/g' -e 's/\(app.kubernetes.io\/version\): "devel"/\1: '${operator_version}'/g' -e 's/\(version\): "devel"/\1: '${operator_version}'/g' -e 's/\("-version"\), "devel"/\1, '${operator_version}'/g' cmd/${platform}/operator/kodata/webhook/*.yaml
    sed -i 's/\(value\): "devel"/\1: '${operator_version}'/g' config/${platform}/base/operator.yaml
  done
}
