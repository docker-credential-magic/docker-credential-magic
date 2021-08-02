#!/usr/bin/env bash

# Find new releases at https://github.com/GoogleCloudPlatform/docker-credential-gcr/releases

set -ex

GCR_HELPER_VERSION="2.0.5"
GCR_HELPER_TARBALL_SHA256="a673b3d6e2fddd0fe6baf807f7b11f98714eb5b901b0c27e26cd33b0bc291ad5"
GCR_HELPER_BINARY_SHA256="2e55d1179811ab1fe9c43334a308dab51a804e8e6557b8d65e4985b23f960d16"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../../

mkdir -p internal/embedded/helpers/embedded/
cd internal/embedded/helpers/embedded/

if [[ ! -f docker-credential-gcr ]]; then
  if [[ ! -f docker-credential-gcr.tar.gz ]]; then
    curl -L -o docker-credential-gcr.tar.gz \
      "https://github.com/GoogleCloudPlatform/docker-credential-gcr/releases/download/v${GCR_HELPER_VERSION}/docker-credential-gcr_linux_amd64-${GCR_HELPER_VERSION}.tar.gz"
  fi
  shasum -a 256 docker-credential-gcr.tar.gz | grep "^${GCR_HELPER_TARBALL_SHA256}  "
  tar -xvf docker-credential-gcr.tar.gz
  rm -f docker-credential-gcr.tar.gz
fi

shasum -a 256 docker-credential-gcr #| grep "^${GCR_HELPER_BINARY_SHA256}  "
