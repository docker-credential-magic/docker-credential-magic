#!/usr/bin/env bash

set -ex

ECR_HELPER_VERSION="0.5.0"
ECR_HELPER_BINARY_SHA256="a0ae9a66b1f41f3312785ec5e17404c7fd2a16a35703c9ea7c050406e20fc503"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../../

mkdir -p internal/embedded/helpers/embedded/
cd internal/embedded/helpers/embedded/

if [[ ! -f docker-credential-ecr-login ]]; then
  curl -L -o docker-credential-ecr-login \
    "https://amazon-ecr-credential-helper-releases.s3.us-east-2.amazonaws.com/${ECR_HELPER_VERSION}/linux-amd64/docker-credential-ecr-login"
fi

shasum -a 256 docker-credential-ecr-login | grep "^${ECR_HELPER_BINARY_SHA256}  "
