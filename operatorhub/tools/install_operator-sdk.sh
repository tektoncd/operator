#!/usr/bin/env bash

function operator_sdk_latest_version() {
  releases_url='https://api.github.com/repos/operator-framework/operator-sdk/releases'
  curl -sL ${releases_url} | jq  -r '.[].tag_name' | sort -Vr | head -n 1
}

dest_dir=${1:-.bin}
[[ ! -d $dest_dir ]] && mkdir -p $dest_dir
echo operator-sdk install - dest_dir: $PWD/$dest_dir
ARCH=$(case $(uname -m) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(uname -m) ;; esac)
OS=$(uname | awk '{print tolower($0)}')
OPERATOR_SDK_VERSION=$(operator_sdk_latest_version)
OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download
OPERATOR_SDK_DL_URL=${OPERATOR_SDK_DL_URL}/${OPERATOR_SDK_VERSION}/operator-sdk_${OS}_${ARCH}
echo operator-sdk download url: ${OPERATOR_SDK_DL_URL}
curl -L -o ${dest_dir}/operator-sdk ${OPERATOR_SDK_DL_URL}
chmod +x ${dest_dir}/operator-sdk
