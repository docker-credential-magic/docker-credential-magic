#!/usr/bin/env bash

# This script is used to generate a brand new
# Azure Container Registry (ACR) resource and credentials
#
# Usage:
#   az login  # (and follow browser auth flow)
#   AZURE_REGISTRY_NAME=uniquereg123 AZURE_RESOURCE_GROUP_NAME=uniquereg123 AZURE_LOCATION=eastus2
#   ./scripts/cloud/generate-azure-registry-creds.sh
#

set -e

if [[ \
      "${AZURE_REGISTRY_NAME}" == "" || \
      "${AZURE_RESOURCE_GROUP_NAME}" == "" || \
      "${AZURE_LOCATION}" == "" \
  ]]; then
  echo "Must set AZURE_REGISTRY_NAME, AZURE_RESOURCE_GROUP_NAME, and AZURE_LOCATION first."
  exit 1
fi
set -x

# 1. Obtain the tenant ID
AZURE_TENANT_ID="$(az account show --query tenantId --output tsv)"

# 2. Create resource group
az group create \
  --name "${AZURE_RESOURCE_GROUP_NAME}" \
  --location "${AZURE_LOCATION}"

# 3. Create registry
az acr create --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
  --name "${AZURE_REGISTRY_NAME}" --sku Basic
RESOURCE_ID="$(az acr show \
  --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
  --name "${AZURE_REGISTRY_NAME}" --query id --output tsv)"
AZURE_REGISTRY_ENDPOINT="$(az acr show \
  --resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
  --name "${AZURE_REGISTRY_NAME}" --query loginServer --output tsv)"

# 4. Create service principal
AZURE_CLIENT_SECRET="$(az ad sp create-for-rbac \
  --name "${AZURE_REGISTRY_NAME}" \
  --skip-assignment \
  --query password --output tsv)"
AZURE_CLIENT_ID="$(az ad sp list \
  --display-name "${AZURE_REGISTRY_NAME}" \
   --query [].appId --output tsv)"

# 5. Grant service principal pull+push access to registry
az role assignment create \
  --assignee "${AZURE_CLIENT_ID}" \
  --scope "${RESOURCE_ID}" \
  --role acrpush

# 6. Print env vars to terminal
set +x
echo
echo
echo "AZURE_REGISTRY_ENDPOINT=${AZURE_REGISTRY_ENDPOINT}"
echo "AZURE_CLIENT_ID=${AZURE_CLIENT_ID}"
echo "AZURE_CLIENT_SECRET=${AZURE_CLIENT_SECRET}"
echo "AZURE_TENANT_ID=${AZURE_TENANT_ID}"
echo