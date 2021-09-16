#!/usr/bin/env bash

# Find new releases at https://github.com/GoogleCloudPlatform/docker-credential-gcr/releases

set -ex

GCR_HELPER_VERSION="2.1.0"
GCR_HELPER_TARBALL_SHA256="91cca7b5ca33133bcd217982be31d670efe7f1a33eb5be72e014f74feecac00f"
GCR_HELPER_BINARY_SHA256="14738e12a09893c25a4952a4661f2e96304d231c4f7f1854e9d9288fcbfecc3e"

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

shasum -a 256 docker-credential-gcr | grep "^${GCR_HELPER_BINARY_SHA256}  "
