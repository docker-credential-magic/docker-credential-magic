#!/usr/bin/env bash

set -ex

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../
export PATH="${PWD}/bin:${PATH}"

PY_REQUIRES="robotframework==4.1"

# Default test settings
export REMOTE_IMAGE="${REMOTE_IMAGE:-ghcr.io/oras-project/oras:v0.12.0@sha256:c17f028ecbe9caed88a352d590fbe58abef4b91baf9157e353f7ab7b60cd6fdb}"
export CONTAINER_NAME="${CONTAINER_NAME:-docker-credential-magic-acceptance}"
export CONTAINER_PORT="${CONTAINER_PORT:-5000}"
export LOCAL_IMAGE="${LOCAL_IMAGE:-localhost:${CONTAINER_PORT}/acceptance:magic2}"
export PUSH_ENTRYPOINT="${PUSH_ENTRYPOINT:-sh}"
export PUSH_MOUNT_FLAGS="${PUSH_MOUNT_FLAGS:--v ${PWD}/testdata/helm/nginx-9.3.6.tgz:/workspace/nginx-9.3.6.tgz}"
export PUSH_ARGS="${PUSH_ARGS:--c 'echo "{}" > config.json && \
                                   oras push --manifest-config config.json:application/vnd.cncf.helm.config.v1+json \
                                   \${REGISTRY_ENDPOINT}/\${REGISTRY_NAMESPACE}:9.3.6 \
                                   nginx-9.3.6.tgz:application/tar+gzip'}"
export POST_PUSH_CMD="${POST_PUSH_CMD:-rm -rf tmp/ && mkdir tmp/}"
export PULL_ENTRYPOINT="${PULL_ENTRYPOINT:-sh}"
export PULL_MOUNT_FLAGS="${PULL_MOUNT_FLAGS:--v ${PWD}/tmp:/workspace}"
export PULL_ARGS="${PULL_ARGS:--c 'oras pull -a \${REGISTRY_ENDPOINT}/\${REGISTRY_NAMESPACE}:9.3.6'}"
export VERIFY_CMD="${VERIFY_CMD:-ls -la tmp/ | grep nginx-9.3.6.tgz}"

# Special case for Google: if GOOGLE_APPLICATION_CREDENTIALS_BASE64 is set,
# create sa.json and add to the mount args
set +x
if [[ "${GOOGLE_APPLICATION_CREDENTIALS_BASE64}" != "" ]]; then
  echo "${GOOGLE_APPLICATION_CREDENTIALS_BASE64}" | base64 -d > sa.json
  export PUSH_MOUNT_FLAGS="${PUSH_MOUNT_FLAGS} -v ${PWD}/sa.json:/sa.json"
  export PULL_MOUNT_FLAGS="${PULL_MOUNT_FLAGS} -v ${PWD}/sa.json:/sa.json"
  export GOOGLE_APPLICATION_CREDENTIALS="/sa.json"
fi
set -x

# Default all provider-specific env vars to empty strings so robot does not complain
set +x
export AZURE_REGISTRY_ENDPOINT="${AZURE_REGISTRY_ENDPOINT}"
export AZURE_REGISTRY_NAMESPACE="${AZURE_REGISTRY_NAMESPACE}"
export AZURE_CLIENT_ID="${AZURE_CLIENT_ID}"
export AZURE_CLIENT_SECRET="${AZURE_CLIENT_SECRET}"
export AZURE_TENANT_ID="${AZURE_TENANT_ID}"
export AWS_REGISTRY_ENDPOINT="${AWS_REGISTRY_ENDPOINT}"
export AWS_REGISTRY_NAMESPACE="${AWS_REGISTRY_NAMESPACE}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}"
export GOOGLE_REGISTRY_ENDPOINT="${GOOGLE_REGISTRY_ENDPOINT}"
export GOOGLE_REGISTRY_NAMESPACE="${GOOGLE_REGISTRY_NAMESPACE}"
export GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS}"
set -x

if [ ! -d .venv/ ]; then
  python3 -m virtualenv --clear .venv/
  .venv/bin/python .venv/bin/pip install $PY_REQUIRES
fi

rm -rf .robot/
.venv/bin/robot --outputdir=.robot/ acceptance/
