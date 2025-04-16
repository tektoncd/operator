#!/usr/bin/env bash

# hack/run-operator-catalog.sh
#
# Run the operator from a catalog image.
# Required environment variables:
#
# - CATALOG_IMG: catalog image to deploy
# - CSV_VERSION: version tag of the cluster service version
#
# Optional environment variables
#
# - KUSTOMIZE_BIN: path to kustomize
# - KUBECTL_BIN: path to kubectl (or equivalent command line)
# - SED_BIN: path to GNU sed
# - CATALOG_NAMESPACE: Namespace to deploy the catalog. Defaults to tektoncd-operator.
# - SUBSCRIPTION_NAMESPACE: Namespace to install the operator via an OLM subscription. Defaults to
#   tektoncd-operator.
# - NAME_PREFIX: prefix to use for all resource names. Defaults to "tektoncd-"

set -eu -o pipefail
echo "Deploy catalog and subscription"

KUSTOMIZE_BIN=${KUSTOMIZE_BIN:-${PWD}/.bin/kustomize}
KUBECTL_BIN=${KUBECTL_BIN:-kubectl}
SED_BIN=${SED_BIN:-sed}
CATALOG_NAMESPACE=${CATALOG_NAMESPACE:-tektoncd-operator}
SUBSCRIPTION_NAMESPACE=${SUBSCRIPTION_NAMESPACE:-tektoncd-operator}
NAME_PREFIX=${NAME_PREFIX:-tektoncd-}

if [ -z "${CATALOG_SOURCE_DIR:-}" ]; then
echo "CATALOG_SOURCE_DIR is not set."
exit 1
fi

if [ -z "${SUBSCRIPTION_DIR:-}" ]; then
echo "SUBSCRIPTION_DIR is not set."
exit 1
fi

if [[ -z ${CATALOG_IMG} ]]; then
    echo "CATALOG_IMG environment variable must be set"
    exit 1
fi

if [[ -z ${CSV_VERSION} ]]; then
    echo "CSV_VERSION environment variable must be set"
    exit 1
fi

function add_kustomizations() {
    echo "Adding replacements not supported by kustomize"
    ${SED_BIN} -i -E "s|image: (.+)$|image: ${CATALOG_IMG}|g" ${CATALOG_SOURCE_DIR}/catalog_source.yaml
    ${SED_BIN} -i -E "s|startingCSV: (.+)$|startingCSV: tektoncd-operator.v${CSV_VERSION}|g" ${SUBSCRIPTION_DIR}/subscription.yaml
    ${SED_BIN} -i -E "s|sourceNamespace: (.+)$|sourceNamespace: ${CATALOG_NAMESPACE}|g" ${SUBSCRIPTION_DIR}/subscription.yaml
    ${SED_BIN} -i -E "s|source: (.+)$|source: ${NAME_PREFIX}operator|g" ${SUBSCRIPTION_DIR}/subscription.yaml

    echo "Applying catalog source and subscription from kustomize"

    pushd ${CATALOG_SOURCE_DIR}
    ${KUSTOMIZE_BIN} edit set namespace "${CATALOG_NAMESPACE}"
    ${KUSTOMIZE_BIN} edit set nameprefix "${NAME_PREFIX}"
    popd

    pushd ${SUBSCRIPTION_DIR}
    ${KUSTOMIZE_BIN} edit set namespace "${SUBSCRIPTION_NAMESPACE}"
    ${KUSTOMIZE_BIN} edit set nameprefix "${NAME_PREFIX}"
    popd
}

function dump_state() {
    echo "Dumping OLM catalog sources"
    ${KUBECTL_BIN} get catalogsources -n "${CATALOG_NAMESPACE}" -o yaml
    echo "Dumping OLM subscriptions"
    ${KUBECTL_BIN} get subscriptions -n "${SUBSCRIPTION_NAMESPACE}" -o yaml
    echo "Dumping OLM installplans"
    ${KUBECTL_BIN} get installplans -n "${SUBSCRIPTION_NAMESPACE}" -o yaml
    echo "Dumping OLM CSVs"
    ${KUBECTL_BIN} get clusterserviceversions -n "${SUBSCRIPTION_NAMESPACE}" -o yaml
    echo "Dumping pods"
    ${KUBECTL_BIN} get pods -n "${SUBSCRIPTION_NAMESPACE}" -o yaml
    echo "${CATALOG_NAMESPACE} -ne ${SUBSCRIPTION_NAMESPACE}"
    if [ "${CATALOG_NAMESPACE}" != "${SUBSCRIPTION_NAMESPACE}" ]; then
        ${KUBECTL_BIN} get pods -n "${CATALOG_NAMESPACE}" -o yaml
    fi
}

function wait_for_pod() {
    label=$1
    namespace=$2
    timeout=$3
    ${KUBECTL_BIN} wait --for=condition=Ready pod -l "${label}" -n "${namespace}" --timeout "${timeout}"
}

add_kustomizations

echo "Deploying catalog source"
${KUSTOMIZE_BIN} build ${CATALOG_SOURCE_DIR} | ${KUBECTL_BIN} apply -f -

echo "Waiting for the catalog source to be ready"
# Wait 15 seconds for the catalog source pod to be created first.
sleep 15
if ! wait_for_pod "olm.catalogSource=${NAME_PREFIX}operator" "${CATALOG_NAMESPACE}" 1m; then
    echo "Failed to deploy catalog source, dumping operator state"
    dump_state
    exit 1
fi

echo "Deploying subscription"
${KUSTOMIZE_BIN} build ${SUBSCRIPTION_DIR} | ${KUBECTL_BIN} apply -f -

echo "Waiting for the operator to be ready"
# Wait 60 seconds for the operator pod to be created
# This needs extra time due to OLM's reconciliation process
sleep 60
if ! wait_for_pod "app=tekton-operator" "${SUBSCRIPTION_NAMESPACE}" 5m; then
    echo "Failed to deploy, dumping operator state"
    dump_state
    exit 1
fi

echo "Deploying Tektoncd Operator"

${KUBECTL_BIN} apply -f - <<EOF
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  profile: basic
  targetNamespace: tekton-pipelines
  pruner:
    resources:
    - pipelinerun
    - taskrun
    keep: 100
    schedule: "0 8 * * *"
EOF

if ! ${KUBECTL_BIN} wait --for=condition=Ready=true tektonconfig.operator.tekton.dev/config --timeout=5m; then
    echo "Failed to deploy Tekton pipelines - dumping state"
    ${KUBECTL_BIN} get tektonconfig.operator.tekton.dev/config -o yaml
    dump_state
    exit 1
fi

exit 0
