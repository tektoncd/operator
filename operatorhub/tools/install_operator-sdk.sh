#!/usr/bin/env sh


dest_dir=${1:-.bin}
[[ ! -d $dest_dir ]] && mkdir -p $dest_dir
echo $dest_dir
# https://sdk.operatorframework.io/docs/installation/#1-download-the-release-binary
export ARCH=$(case $(uname -m) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(uname -m) ;; esac)
export OS=$(uname | awk '{print tolower($0)}')
export OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/v1.8.0
curl -L -o ${dest_dir}/operator-sdk ${OPERATOR_SDK_DL_URL}/operator-sdk_${OS}_${ARCH}
chmod +x ${dest_dir}/operator-sdk
