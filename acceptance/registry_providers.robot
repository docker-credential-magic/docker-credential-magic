*** Settings ***
Documentation     Tests to verify that docker-credential-magic/magician works properly
...               against various commercial registry providers (Docker required).
Library           String
Library           OperatingSystem
Library           lib/Sh.py
Suite Setup       Suite Setup
Suite Teardown    Suite Teardown

*** Test Cases ***
Azure Container Registry (ACR)
    Cleanup envfile
    Skip test or add to envfile   AZURE_REGISTRY_ENDPOINT   REGISTRY_ENDPOINT
    Skip test or add to envfile   AZURE_REGISTRY_NAMESPACE  REGISTRY_NAMESPACE
    Skip test or add to envfile   AZURE_CLIENT_ID
    Skip test or add to envfile   AZURE_CLIENT_SECRET
    Skip test or add to envfile   AZURE_TENANT_ID
    Test registry provider integration

Amazon Elastic Container Registry (ECR)
    Cleanup envfile
    Skip test or add to envfile   AWS_REGISTRY_ENDPOINT   REGISTRY_ENDPOINT
    Skip test or add to envfile   AWS_REGISTRY_NAMESPACE  REGISTRY_NAMESPACE
    Skip test or add to envfile   AWS_DEFAULT_REGION
    Skip test or add to envfile   AWS_ACCESS_KEY_ID
    Skip test or add to envfile   AWS_SECRET_ACCESS_KEY
    Test registry provider integration

Google Container Registry (GCR) / Google Artifact Registry (GAR)
    Cleanup envfile
    Skip test or add to envfile   GOOGLE_REGISTRY_ENDPOINT  REGISTRY_ENDPOINT
    Skip test or add to envfile   GOOGLE_REGISTRY_NAMESPACE  REGISTRY_NAMESPACE
    Skip test or add to envfile   GOOGLE_APPLICATION_CREDENTIALS
    Test registry provider integration

*** Keyword ***
Skip test or add to envfile
    [Arguments]   ${key}    ${override_key}=unset
    Skip If   "%{${key}}" == ""   Missing required env var ${key}
    IF   "${override_key}" != "unset"
        Add to envfile   "${override_key}"   "%{${key}}"
    ELSE
        Add to envfile   "${key}"   "%{${key}}"
    END

Add to envfile
    [Arguments]   ${key}   ${value}
    Should pass no output   echo "${key}=${value}" >> test.env

Cleanup envfile
    Should pass   rm -f test.env || true

Test registry provider integration
    Should pass   docker run --rm --env-file=test.env --entrypoint %{PUSH_ENTRYPOINT} %{PUSH_MOUNT_FLAGS} %{LOCAL_IMAGE} %{PUSH_ARGS}
    Should pass   %{POST_PUSH_CMD}
    Should pass   docker run --rm --env-file=test.env --entrypoint %{PULL_ENTRYPOINT} %{PULL_MOUNT_FLAGS} %{LOCAL_IMAGE} %{PULL_ARGS}
    Should pass   %{VERIFY_CMD}

Start local test registry
    Should pass   docker rmi -f %{LOCAL_IMAGE} || true
    Should pass   docker rm -f %{CONTAINER_NAME} || true
    Should pass   docker run --rm -d -p %{CONTAINER_PORT}:5000 --name %{CONTAINER_NAME} registry

Stop local test registry
    Should pass   docker logs %{CONTAINER_NAME}
    Should pass   docker rm -f %{CONTAINER_NAME}

Mutate remote test image
    Should pass   docker-credential-magician mutate %{REMOTE_IMAGE} -t %{LOCAL_IMAGE}
    Should pass   docker pull %{LOCAL_IMAGE}
    Should pass   docker run --rm --entrypoint sh %{LOCAL_IMAGE} -c 'ls -la /opt/magic/bin'
    Should pass   docker run --rm --entrypoint /opt/magic/bin/docker-credential-magic %{LOCAL_IMAGE} version || true
    Should pass   docker run --rm --entrypoint sh %{LOCAL_IMAGE} -c 'echo "example.com" | /opt/magic/bin/docker-credential-magic get' || true
    Should pass   docker run --rm --entrypoint sh %{LOCAL_IMAGE} -c 'apk update && apk add file && file /opt/magic/bin/docker-credential-magic' || true

Suite Setup
   Start local test registry
   Mutate remote test image

Suite Teardown
    Cleanup envfile
    Stop local test registry
