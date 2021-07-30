## Development

### Download Go dependencies

Run the following:

```
make vendor
```

### Generating embedded content

First, fetch all credential helper binaries into `pkg/magician/credential-helpers/`:

```
make fetch-helpers
```

Next, copy over the default mappings file into  `pkg/magician/` and `cmd/docker-credential-magic/`:

```
make copy-mappings
```

Finally, build our custom `magic` helper into `pkg/embedded/helpers/embedded/`:

```
make build-magic-embedded
```

Note: All embedded helpers are for Linux amd64 architecture.

### Building magician binary

The `magician` binary is built with all supported credential helpers baked in.

The downside is that our binary is larger that normal (~30mb), but the upside
is that users will not need to make any network requests (to fetch credential helpers)
in order to use this tool.

After running the steps above related to embedded content,
run the following:

```
make build-magician
```

This will build a binary for you system architecture at `bin/docker-credential-magician`.

### Building a locally-usable magic binary

You may wish to have a `magic` binary to use locally.

To do so, run the following:

```
make build-magic
```

This will build a binary for you system architecture at `bin/docker-credential-magic`.

### Run unit tests

Run the following:

```
make test
```

This will produce an HTML coverage report at `.cover/coverage.html`.

### Run acceptance tests

First, make sure the Docker daemon is running locally, and `virtualenv` is installed.

Next, set env vars related to the registry providers you wish to test:

```
export AZURE_REGISTRY_ENDPOINT="..."
export AZURE_REGISTRY_NAMESPACE="..."
export AZURE_CLIENT_ID="..."
export AZURE_CLIENT_SECRET="..."
export AZURE_TENANT_ID="..."
export AWS_REGISTRY_ENDPOINT="..."
export AWS_REGISTRY_NAMESPACE="..."
export AWS_DEFAULT_REGION="..."
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export GOOGLE_REGISTRY_ENDPOINT="..."
export GOOGLE_REGISTRY_NAMESPACE="..."
export GOOGLE_APPLICATION_CREDENTIALS_BASE64="..."
```

If vars are missing for a registry provider, then related tests will be skipped.

Finally, run the following:

```
make acceptance
```

This will produce an HTML test report at `.robot/report.html`.

### Clean workspace

To remove all generated content etc., run the following:

```
make clean
```
