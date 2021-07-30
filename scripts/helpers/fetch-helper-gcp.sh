#!/usr/bin/env bash

set -ex

GCR_HELPER_VERSION="2.0.5"
GCR_HELPER_TARBALL_SHA256="0019dfc4b32d63c1392aa264aed2253c1e0c2fb09216f8e2cc269bbfb8bb49b5"
GCR_HELPER_BINARY_SHA256="716dd54138618abefe02e40197240864500f204ca58668295c49d8a72efbaae1"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../../

mkdir -p pkg/magician/credential-helpers/
cd pkg/magician/credential-helpers/

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
