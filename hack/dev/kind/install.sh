#!/usr/bin/env bash

set -euf
cd $(dirname $(readlink -f ${0}))

export KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
export KUBECONFIG=${HOME}/.kube/config.${KIND_CLUSTER_NAME}
export TARGET=kubernetes

kind=$(type -p kind)
[[ -z ${kind} ]] && { echo "Install kind"; exit 1 ;}

TMPD=$(mktemp -d /tmp/.GITXXXX)
REG_PORT='5000'
REG_NAME='kind-registry'
SUDO=sudo

[[ $(uname -s) == "Darwin" ]] && {
    SUDO=
}

# cleanup on exit (useful for running locally)
cleanup() { rm -rf ${TMPD} ;}
trap cleanup EXIT

function start_registry() {
    running="$(docker inspect -f '{{.State.Running}}' ${REG_NAME} 2>/dev/null || echo false)"

    if [[ ${running} != "true" ]];then
        docker rm -f kind-registry || true
        docker run \
               -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" \
               --name "${REG_NAME}" \
               registry:2
    fi
}

function reinstall_kind() {
	${SUDO} $kind delete cluster --name ${KIND_CLUSTER_NAME} || true
	sed "s,%DOCKERCFG%,${HOME}/.docker/config.json,"  kind.yaml > ${TMPD}/kconfig.yaml

       cat <<EOF >> ${TMPD}/kconfig.yaml
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REG_PORT}"]
    endpoint = ["http://${REG_NAME}:5000"]
EOF

	${SUDO} ${kind} create cluster --name ${KIND_CLUSTER_NAME} --config  ${TMPD}/kconfig.yaml
	mkdir -p $(dirname ${KUBECONFIG})
	${SUDO} ${kind} --name ${KIND_CLUSTER_NAME} get kubeconfig > ${KUBECONFIG}


    docker network connect "kind" "${REG_NAME}" 2>/dev/null || true
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
