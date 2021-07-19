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
