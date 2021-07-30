#!/usr/bin/env bash

set -ex

GCR_HELPER_VERSION="2.0.4"
GCR_HELPER_TARBALL_SHA256="4fca8441c41802f4bcc4912672c55d4b1232decb90639f8a684d3b389e4e6e91"
GCR_HELPER_BINARY_SHA256="716dd54138618abefe02e40197240864500f204ca58668295c49d8a72efbaae1"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../../

mkdir -p pkg/embedded/helpers/embedded/
cd pkg/embedded/helpers/embedded/

if [[ ! -f docker-credential-gcr ]]; then
  if [[ ! -f docker-credential-gcr.tar.gz ]]; then
    curl -L -o docker-credential-gcr.tar.gz \
      "https://github.com/GoogleCloudPlatform/docker-credential-gcr/releases/download/v${GCR_HELPER_VERSION}/docker-credential-gcr_linux_amd64-${GCR_HELPER_VERSION}.tar.gz"
  fi
  shasum -a 256 docker-credential-gcr.tar.gz | grep "^${GCR_HELPER_TARBALL_SHA256}  "
  tar -xvf docker-credential-gcr.tar.gz
  rm -f docker-credential-gcr.tar.gz
fi

shasum -a 256 docker-credential-gcr | grep "^${GCR_HELPER_BINARY_SHA256}  "
