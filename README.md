# docker-credential-magic

[![GitHub Actions status](https://github.com/docker-credential-magic/docker-credential-magic/workflows/build/badge.svg)](https://github.com/docker-credential-magic/docker-credential-magic/actions?query=workflow%3Abuild+)
[![GoDoc](https://godoc.org/github.com/docker-credential-magic/docker-credential-magic?status.svg)](https://godoc.org/github.com/docker-credential-magic/docker-credential-magic)

![docker-credential-magic](./docker-credential-magic.png)

- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
  - [How to use `docker-credential-magic`](#how-to-use-docker-credential-magic)
    - [Local setup](#local-setup)
  - [How to use `docker-credential-magician`](#how-to-use-docker-credential-magician)
    - [Including a subset of helpers](#including-a-subset-of-helpers)
    - [Using custom mappings and/or helpers](#using-custom-mappings-andor-helpers)
    - [Go library](#go-library)
- [Project history](#project-history)
- [Contributing](#contributing)
  - [Adding support for a new helper](#adding-support-for-a-new-helper)

## Overview

This repo contains the source for two separate tools:

- `docker-credential-magic` - credential helper which proxies auth to other helpers based on domain name
- `docker-credential-magician` - tool to augment images with various credential helpers (including `magic`)

The following third-party Docker credential helpers are currently supported:

- **`azure`** (via [`docker-credential-acr-env`](https://github.com/chrismellard/docker-credential-acr-env)) - for Azure Container Registry (ACR)
- **`aws`** (via [`docker-credential-ecr-login`](https://github.com/awslabs/amazon-ecr-credential-helper)) - for Amazon Elastic Container Registry (ECR)
- **`gcp`** (via [`docker-credential-gcr`](https://github.com/GoogleCloudPlatform/docker-credential-gcr)) - for Google Container Registry (GCR),
  Google Artifact Registry (GAR)

## Installation

Download [latest release](https://github.com/docker-credential-magic/docker-credential-magic/releases/latest) tarball
for your system and install both tools manually:

```
cat docker-credential-magic*.tar.gz | tar x -C /usr/local/bin 'docker-credential-magic*'
```

## Usage

### How to use `docker-credential-magic`

When using for the first time, initialize the configuration:

```
docker-credential-magic init
```

Then:

```
$ echo <domain> | docker-credential-magic get
```

---

The following example shows how `docker-credential-magic` can be used to
proxy auth to `docker-credential-gcr`, based on the detection of a `*.gcr.io` domain:

*Note: Example requires [`docker-credential-gcr`](https://github.com/GoogleCloudPlatform/docker-credential-gcr)
to be pre-installed*

```
$ export GOOGLE_APPLICATION_CREDENTIALS="${PWD}/service-account-key.json"
```

```
$ echo "us.gcr.io" | docker-credential-magic get
{"ServerURL":"us.gcr.io","Username":"_dcgcr_token","Secret":"*****"}
```

*Note: `docker-credential-magic` is a "read-only" credential helper, and does not modify credentials in any way (some helpers implement other subcommands like `store` or `erase`).*

#### Local setup

The primary purpose of `magic` is to be added to images via `magician`.
However, you may wish to also use this tool on your local machine.

It is required that mappings files for each supported helper are present on
disk. The environment variable `DOCKER_CREDENTIAL_MAGIC_CONFIG` is used by `magic`
to find these files, nested under an `etc/` subdirectory.

If `DOCKER_CREDENTIAL_MAGIC_CONFIG` is not set, `magic` respects the
[XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html),
looking for mappings files under `$XDG_CONFIG_HOME/magic/etc/`.
This is equivalent to the following on each operating system:

- Linux: `$HOME/.config/magic/etc/`
- macOS: `$HOME/Library/Application Support/magic/etc`
- Windows: `%APPDATA%\magic\etc`

`magic` has a useful subcommand, `init`, which will auto-create this directory
and populate it with the default mappings files, as well as a catch-all
Docker `config.json` file in the parent directory:

```
$ docker-credential-magic init
Creating directory '/Users/me/Library/Application Support/magic/etc' ...
Creating mapping file '/Users/me/Library/Application Support/magic/etc/aws.yml' ...
Creating mapping file '/Users/me/Library/Application Support/magic/etc/azure.yml' ...
Creating mapping file '/Users/me/Library/Application Support/magic/etc/gcp.yml' ...
Creating magic config file '/Users/me/Library/Application Support/magic/config.json' ...
```

`magic` has another subcommand, `home`, which you can use to
modify the `DOCKER_CONFIG` env var to point to the magic directory:

```
$ export DOCKER_CONFIG="$(docker-credential-magic home)"
```

You may wish to add the previous command to your `~/.bashrc` / `~/.bash_profile`.

If no matching domains are found, `magic` will fall back to use
your existing `$HOME/.docker/config.json`.

Note: At this time, `magic` will not automatically install the supported
helpers on your machine. You should install each of these manually.
For example, to install `ecr-login` on macOS via Homebrew:

```
$ brew install docker-credential-helper-ecr
```

### How to use `docker-credential-magician`

```
$ docker-credential-magician mutate <ref>
```

---

The following example shows how `docker-credential-magician` can be used to
(1) augment the [`cosign`](https://github.com/sigstore/cosign) image with
various credential helpers, (2) set the default credential store to `magic`,
and (3) push the new image to a registry running at `localhost:5000`:

```
$ docker-credential-magician mutate \
    gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0 \
    -t localhost:5000/cosign:v0.5.0-magic
2021/07/29 17:06:59 Pulling gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0 ...
2021/07/29 17:07:01 Adding /opt/magic/etc/aws.yml ...
2021/07/29 17:07:01 Adding /opt/magic/etc/azure.yml ...
2021/07/29 17:07:01 Adding /opt/magic/etc/gcp.yml ...
2021/07/29 17:07:01 Adding /opt/magic/bin/docker-credential-ecr-login ...
2021/07/29 17:07:01 Adding /opt/magic/bin/docker-credential-acr-env ...
2021/07/29 17:07:01 Adding /opt/magic/bin/docker-credential-gcr ...
2021/07/29 17:07:01 Adding /opt/magic/bin/docker-credential-magic ...
2021/07/29 17:07:01 Adding /opt/magic/config.json ...
2021/07/29 17:07:02 Prepending PATH with /opt/magic/bin ...
2021/07/29 17:07:02 Setting DOCKER_CONFIG to /opt/magic ...
2021/07/29 17:07:02 Setting DOCKER_CREDENTIAL_MAGIC_CONFIG to /opt/magic ...
2021/07/29 17:07:02 Pushing image to localhost:5000/cosign:v0.5.0-magic ...
2021/07/29 17:07:04 Done.
```

```
$ docker run --rm --entrypoint sh \
    localhost:5000/cosign:v0.5.0-magic \
    -c 'ls -lah /opt/magic/etc && \
        ls -lah /opt/magic/bin &&
        env | grep DOCKER_ &&
        cat $DOCKER_CONFIG/config.json'
total 20K
drwxr-xr-x    2 root     root        4.0K Jul 29 21:00 .
drwxr-xr-x    4 root     root        4.0K Jul 29 21:00 ..
-r-xr-xr-x    1 root     root          45 Jan  1  1970 aws.yml
-r-xr-xr-x    1 root     root          40 Jan  1  1970 azure.yml
-r-xr-xr-x    1 root     root          44 Jan  1  1970 gcp.yml
total 25M
drwxr-xr-x    2 root     root        4.0K Jul 29 21:00 .
drwxr-xr-x    4 root     root        4.0K Jul 29 21:00 ..
-r-xr-xr-x    1 root     root        8.7M Jan  1  1970 docker-credential-acr-env
-r-xr-xr-x    1 root     root        7.8M Jan  1  1970 docker-credential-ecr-login
-r-xr-xr-x    1 root     root        5.6M Jan  1  1970 docker-credential-gcr
-r-xr-xr-x    1 root     root        3.0M Jan  1  1970 docker-credential-magic
DOCKER_CREDENTIAL_MAGIC_CONFIG=/opt/magic
DOCKER_CONFIG=/opt/magic
{"credsStore":"magic"}
```

If the `-t` / `--tag` flag is not provided, `magician` will default to
publishing the image back to its original location (overwriting the existing tag).

*Note: At this time, `docker-credential-magician` is only designed for x86–64/AMD64 Linux containers.
More platforms may be supported in the future.*

#### Including a subset of helpers

You may specify the `-i` / `--include` flag (one or more times) to
limit the helpers that are added to the image.

For example, to only include the `azure` and `gcp` helpers:

```
$ docker-credential-magician mutate example.com/myimage:1.2.3 \
    -i azure -i gcp
```

Note: These each must match one of the supported helpers found in the
[mappings/](./mappings/) directory.

#### Using custom mappings and/or helpers

In some scenarios, you may wish to supply a custom directory of mappings,
for example to add extra domains. Or, you may wish to supply a custom
directory of helper binaries, if you need to use a different version of
a helper, or add your own.

For these cases, you can use the following flags:

- `--mappings-dir <custom_mappings_dir>`
- `--helpers-dir <custom_helpers_dir>`

Please note that all mappings and helpers must be provided (as in, `magician` will
not automatically resolve any missing binaries in `<custom_helpers_dir>`).

In addition, all helpers in `<custom_helpers_dir>` must be built for
a Linux amd64 architecture.

Lastly, the `magic` helper will *always* be sourced from
the one baked into `magician`.

#### Go library

You may wish to make use of `magician` functionality in your Go application.

Here is a Go example which mimics the command-line example found above:

```go
package main

import (
	"github.com/docker-credential-magic/docker-credential-magic/pkg/magician"
)

func main() {
	src := "gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0"
	dst := "localhost:5000/cosign:v0.5.0-magic"

	err := magician.Mutate(src, magician.MutateOptWithTag(dst))
	if err != nil {
		panic(err)
	}
}
```

Note: since `magician` makes use of embedded files (helper binaries and mappings), if you try
running this example, you will see errors such as the following:

```
pattern embedded/*: no matching files found
```

To fix this, you must manually populate these directories in your `GOPATH`
after running `go mod tidy`:

```
git clone https://github.com/docker-credential-magic/docker-credential-magic.git tmp/
pushd tmp/
make clean vendor fetch-helpers copy-mappings build-magic-embedded
popd
for d in $(find "$(go env | grep GOPATH | awk -F "=" '{print $2}' | tr -d '"')/pkg/mod/github.com/docker-credential-magic" -mindepth 1 -maxdepth 1 -type d); do
  sudo cp -r tmp/internal/embedded/helpers/embedded "${d}/internal/embedded/helpers" || true
  sudo cp -r tmp/internal/embedded/mappings/embedded "${d}/internal/embedded/mappings" || true
done
rm -rf tmp/
```

(If there is an easier way to approach this, please let us know)

## Project history

The original concept for this project and its design
can be found in the following GitHub conversations:

- [google/ko/issues/3#issuecomment-749695477](https://github.com/google/ko/issues/3#issuecomment-749695477)
- [google/go-containerregistry/issues/1059#issuecomment-867136469](https://github.com/google/go-containerregistry/issues/1059#issuecomment-867136469)

If you are interested in understanding how Docker credential helpers work,
you may enjoy
[this image](https://raw.githubusercontent.com/google/go-containerregistry/main/images/credhelper-basic.svg).

## Contributing

Contributions are welcome!

Prior to submitting a
[pull request](https://github.com/docker-credential-magic/docker-credential-magic/pulls),
please check the list of
[open issues](https://github.com/docker-credential-magic/docker-credential-magic/issues).
If there is not an existing issue related to your changes, please open a
new issue to first discuss your thoughts with the project maintainers.

### Adding support for a new helper

If you are contributing support for another helper, here are the necessary steps:

- [ ] Decide on a unique slug to represent your helper (e.g. `cats`)
- [ ] Create a valid mappings file at `mappings/<slug>.yml`
- [ ] Create a script to download your helper at `scripts/helpers/fetch-helper-<slug>.sh`
- [ ] Update the project README to declare support for your helper (and update output in code snips)
- [ ] If possible, add acceptance tests for your helper. The following are relevant files:
  - [`.github/workflows/build.yml`](./.github/workflows/build.yml)
  - [`scripts/acceptance.sh`](./scripts/acceptance.sh)
  - [`acceptance/registry_providers.robot`](./acceptance/registry_providers.robot)
  - [`DEVELOPMENT.md`](./DEVELOPMENT.md)

For more info on project development, please see [this page](./DEVELOPMENT.md).
