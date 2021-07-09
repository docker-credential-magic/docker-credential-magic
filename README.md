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

#### Examples

##### cosign

The following is an example of augmenting the image for [cosign](https://github.com/sigstore/cosign):

```
$ bin/docker-credential-magic gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0
Loaded image: gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0.magic
```

```
$ docker run --rm \
    gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0.magic -h
USAGE
  cosign [flags] <subcommand>

SUBCOMMANDS
  verify             Verify a signature on the supplied container image
  sign               Sign the supplied container image.
  upload             upload signatures to the supplied container image
  generate           generate (usigned) signature payloads from the supplied container image
  download           Download signatures from the supplied container image
  generate-key-pair  generate-key-pair generates a key-pair
  sign-blob          Sign the supplied blob, outputting the base64-encoded signature to stdout.
  upload-blob        Upload one or more blobs to the supplied container image address
  copy               Copy the supplied container image and signatures.
  clean              Remove all signatures from an image
  verify-blob        Verify a signature on the supplied blob
  triangulate        Outputs the located cosign image reference. This is the location cosign stores signatures.
  version            Prints the cosign version
  public-key         public-key gets a public key from the key-pair


FLAGS
  -d false          log debug output
  -output-file ...  log output to a file
```

```
$ docker run --rm --entrypoint docker-credential-gcr \
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

##### oras

The following is an example of augmenting the image for [oras](https://github.com/oras-project/oras):

```
$ bin/docker-credential-magic ghcr.io/oras-project/oras:v0.12.0
Loaded image: ghcr.io/oras-project/oras:v0.12.0.magic
```

```
$ docker run --rm \
    ghcr.io/oras-project/oras:v0.12.0.magic -h
Usage:
  oras [command]

Available Commands:
  help        Help about any command
  login       Log in to a remote registry
  logout      Log out from a remote registry
  pull        Pull files from remote registry
  push        Push files to remote registry
  version     Show the oras version information

Flags:
  -h, --help   help for oras

Use "oras [command] --help" for more information about a command.
```

```
$ docker run --rm --entrypoint docker-credential-gcr \
    ghcr.io/oras-project/oras:v0.12.0.magic help
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

```
$ docker run -it --rm --entrypoint sh \
    -v ${PWD}/testdata/helm/nginx-9.3.6.tgz:/workspace/nginx-9.3.6.tgz \
    -v ${PWD}/sa.json:/sa.json \
    -e GOOGLE_APPLICATION_CREDENTIALS=/sa.json \
    ghcr.io/oras-project/oras:v0.12.0.magic
/workspace # mkdir ~/.docker
/workspace # echo '{"credHelpers": {"us-central1-docker.pkg.dev": "gcr"}}' > ~/.docker/config.json
/workspace # echo '{}' > config.json
/workspace # oras push --manifest-config config.json:application/vnd.cncf.helm.config.v1+json \
                us-central1-docker.pkg.dev/docker-credential-magic/demo/nginx:9.3.6 \
                nginx-9.3.6.tgz:application/tar+gzip
```