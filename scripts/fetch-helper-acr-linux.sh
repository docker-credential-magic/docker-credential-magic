#!/usr/bin/env bash

set -ex

# Note: these checksums are subject to change,
# and they are not available on a GitHub release page
ACR_HELPER_TARBALL_SHA256="9c3badc30536a0a928064b587a9ac1fe842041047e27bf9300cfda4deaf3a12f"
ACR_HELPER_BINARY_SHA256="7e266408a50b4bdeeb55008ded0711c4b7de6390fac359d06aeb79fc51efa7eb"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../

mkdir -p credential-helpers/
cd credential-helpers/

if [[ ! -f docker-credential-acr-linux ]]; then
  if [[ ! -f docker-credential-acr-linux-amd64.tar.gz ]]; then
    curl -L -o docker-credential-acr-linux-amd64.tar.gz \
      "https://aadacr.blob.core.windows.net/acr-docker-credential-helper/docker-credential-acr-linux-amd64.tar.gz"
  fi
  shasum -a 256 docker-credential-acr-linux-amd64.tar.gz | grep "^${ACR_HELPER_TARBALL_SHA256}  "
  tar -xvf docker-credential-acr-linux-amd64.tar.gz
  rm -f config-edit # this file is not needed
fi

shasum -a 256 docker-credential-acr-linux | grep "^${ACR_HELPER_BINARY_SHA256}  "
