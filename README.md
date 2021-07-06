# docker-credential-magic
Tool to augment existing images with various Docker credential helpers

## Development

### Generating `pkged.go`

First, fetch all credential helper binaries into `credential-helpers/`:

```
make fetch-helpers
```

Next, run our custom [pkger](https://github.com/markbates/pkger) script:

```
make pkger-gen
```

This will create the file `pkged.go` in `cmd/docker-credential-magic/`, which
allows us to build a single binary with all supported credential helpers baked in.

The downside is that our binary is larger that normal (~30mb), but the upside
is that users will not need to make any network requests (to fetch credential helpers)
in order to use this tool.

### Building binary

After running the steps above related to `pkged.go`,
run the following:

```
make vendor build
```

### Run binary

```
bin/docker-credential-magic <ref>
```

Which will produce a new image in your local Docker engine named the following:

```
<ref>.magic
```

#### Example

```
$ bin/docker-credential-magic gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0
Loaded image: gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0.magic
```

```
$ docker run -it --rm --entrypoint docker-credential-gcr \
    gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0.magic help
Usage: docker-credential-gcr <flags> <subcommand> <subcommand args>

Subcommands:
	clear            remove all stored credentials
	commands         list all command names
	help             describe subcommands and their syntax
	version          print the version of the binary to stdout

Subcommands for Config:
	config           configure the credential helper
	configure-docker  configures the Docker client to use docker-credential-gcr

Subcommands for Docker credential store API:
	erase            (UNIMPLEMENTED) erase any stored credentials for the server specified via stdin
	get              for the server specified via stdin, return the stored credentials via stdout
	list             (UNIMPLEMENTED) list all stored credentials
	store            (UNIMPLEMENTED) for the specified server, store the credentials provided via stdin

Subcommands for GCR authentication:
	gcr-login        log in to GCR
	gcr-logout       log out from GCR

```