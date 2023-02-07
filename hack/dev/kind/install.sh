#!/usr/bin/env bash

set -euf
cd $(dirname $(readlink -f ${0}))

export CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-docker}
export KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
export KUBECONFIG=${HOME}/.kube/config.${KIND_CLUSTER_NAME}
export TARGET=kubernetes

kind=$(type -p kind)
[[ -z ${kind} ]] && { echo "Install kind"; exit 1 ;}

TMPD=$(mktemp -d /tmp/.GITXXXX)
REG_PORT='5000'
REG_NAME='kind-registry'
SUDO=sudo
HOST_HTTP_PORT=80
HOST_HTTPS_PORT=443

[[ $(uname -s) == "Darwin" ]] && {
    SUDO=
}

CONTAINER_RUNTIME_ARGS=""
CONTAINER_RUNTIME_CONFIG="${HOME}/.docker/config.json"
if [ "$CONTAINER_RUNTIME" = "podman" ]; then
    SUDO=
    # https://github.com/containers/podman/issues/15664
    CONTAINER_RUNTIME_ARGS="--net=kind"
    CONTAINER_RUNTIME_CONFIG="${XDG_RUNTIME_DIR}/containers/auth.json"
    HOST_HTTP_PORT=7080
    HOST_HTTPS_PORT=7443
fi

# cleanup on exit (useful for running locally)
cleanup() { rm -rf ${TMPD} ;}
trap cleanup EXIT

function start_registry() {
    running="$(${CONTAINER_RUNTIME} inspect -f '{{.State.Running}}' ${REG_NAME} 2>/dev/null || echo false)"

    if [[ ${running} != "true" ]];then
        ${CONTAINER_RUNTIME} rm -f kind-registry || true
        ${CONTAINER_RUNTIME} run \
            ${CONTAINER_RUNTIME_ARGS} \
            -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" \
            --name "${REG_NAME}" \
            registry:2
    fi
}

function reinstall_kind() {
	${SUDO} $kind delete cluster --name ${KIND_CLUSTER_NAME} || true
  sed "s,_DOCKERCFG_,${CONTAINER_RUNTIME_CONFIG},"  kind.yaml > ${TMPD}/kconfig.yaml
  sed -i "s,_HOST_HTTP_PORT_,${HOST_HTTP_PORT}," ${TMPD}/kconfig.yaml
  sed -i "s,_HOST_HTTPS_PORT_,${HOST_HTTPS_PORT}," ${TMPD}/kconfig.yaml

  cat <<EOF >> ${TMPD}/kconfig.yaml
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REG_PORT}"]
    endpoint = ["http://${REG_NAME}:5000"]
EOF

	${SUDO} ${kind} create cluster --name ${KIND_CLUSTER_NAME} --config  ${TMPD}/kconfig.yaml
	mkdir -p $(dirname ${KUBECONFIG})
	${SUDO} ${kind} --name ${KIND_CLUSTER_NAME} get kubeconfig > ${KUBECONFIG}

  # https://github.com/containers/podman/issues/15664
  if [ "$CONTAINER_RUNTIME" = "docker" ]; then
    ${CONTAINER_RUNTIME} network connect "kind" "${REG_NAME}" 2>/dev/null || true
  fi
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

}

main() {
  start_registry
	reinstall_kind
	export KUBECONFIG=${HOME}/.kube/config.${KIND_CLUSTER_NAME}
	export KO_DOCKER_REPO=localhost:5000
	cd ..
	cd ..
	cd ..
	make apply

	echo "##############################################"
	echo "##############################################"
  echo "kubeconfig location: ${KUBECONFIG}"
	echo "Run => export KUBECONFIG=${KUBECONFIG}"
	echo "##############################################"
	echo "##############################################"
}

main
