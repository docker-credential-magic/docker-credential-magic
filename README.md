# docker-credential-magic

[![GitHub Actions status](https://github.com/jdolitsky/docker-credential-magic/workflows/build/badge.svg)](https://github.com/jdolitsky/docker-credential-magic/actions?query=workflow%3Abuild+)

![docker-credential-magic](./docker-credential-magic.png)

This repo contains the source for two separate tools:

- `docker-credential-magic` - credential helper which proxies auth to other helpers based on domain name
- `docker-credential-magician` - tool to augment images with various credential helpers (including `magic`)

The following third-party Docker credential helpers are currently supported:

- [`acr-env`](https://github.com/chrismellard/docker-credential-acr-env) - for Azure Container Registry (ACR)
- [`ecr-login`](https://github.com/awslabs/amazon-ecr-credential-helper) - for Amazon Elastic Container Registry (ECR)
- [`gcr`](https://github.com/GoogleCloudPlatform/docker-credential-gcr) - for Google Container Registry (GCR),
  Google Artifact Registry (GAR)

## Installation

Download [latest release](https://github.com/jdolitsky/docker-credential-magic/releases/latest) tarball
for your system and install both tools manually:

```
cat docker-credential-magic*.tar.gz | \
  tar x -C /usr/local/bin 'docker-credential-magic*'
```

## Usage

### `docker-credential-magic`

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

### `docker-credential-magician`

The following example shows how `docker-credential-magician` can be used to
augment the [`cosign`](https://github.com/sigstore/cosign) image with
various credential helpers, and set the default credential store to `magic`:

*Note: Requires local Docker daemon to be running*

```
$ docker-credential-magician gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0
2021/07/19 15:49:11 Loaded image: gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0.magic
```

```
$ docker run --rm --entrypoint sh \
    gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0.magic \
    -c 'ls -lah /opt/magic/bin &&
        env | grep DOCKER_CONFIG &&
        cat $DOCKER_CONFIG/config.json'
drwxr-xr-x    2 root     root        4.0K Jul 19 19:49 .
drwxr-xr-x    3 root     root        4.0K Jul 19 19:49 ..
-r-xr-xr-x    1 root     root        8.7M Jan  1  1970 docker-credential-acr-env
-r-xr-xr-x    1 root     root        7.8M Jan  1  1970 docker-credential-ecr-login
-r-xr-xr-x    1 root     root        5.6M Jan  1  1970 docker-credential-gcr
-r-xr-xr-x    1 root     root        2.2M Jan  1  1970 docker-credential-magic
DOCKER_CONFIG=/opt/magic
{"credsStore":"magic"}
```
