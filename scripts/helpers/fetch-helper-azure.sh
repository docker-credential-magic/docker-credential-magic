#!/usr/bin/env bash

# Find new releases at https://github.com/chrismellard/docker-credential-acr-env/releases

set -ex

ACR_HELPER_VERSION="0.6.0"
ACR_HELPER_TARBALL_SHA256="97a2d8079317dcc6807347689a6775779d31e1f745890aca270429bc1ad3fe11"
ACR_HELPER_BINARY_SHA256="98ea9e979fd9a1094209b39f783e6a4d8c5d864f979d8078cdc348e2c6d39530"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR/../../

mkdir -p internal/embedded/helpers/embedded/
cd internal/embedded/helpers/embedded/

if [[ ! -f docker-credential-acr-env ]]; then
  TAR_FILENAME="docker-credential-acr-env_${ACR_HELPER_VERSION}_Linux_x86_64.tar.gz"
  if [[ ! -f "${TAR_FILENAME}" ]]; then
    curl -L -o "${TAR_FILENAME}" \
      "https://github.com/chrismellard/docker-credential-acr-env/releases/download/${ACR_HELPER_VERSION}/${TAR_FILENAME}"
  fi
  shasum -a 256 "${TAR_FILENAME}" | grep "^${ACR_HELPER_TARBALL_SHA256}  "
  tar -xvf "${TAR_FILENAME}"
  rm -f LICENSE README.md # these files are not needed
  rm -f "${TAR_FILENAME}"
fi

shasum -a 256 docker-credential-acr-env | grep "^${ACR_HELPER_BINARY_SHA256}  "
