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
make pkger
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
make build
```

### Run binary

```
bin/docker-credential-magic <ref>
```

Which will produce a new image in your local Docker engine named the following:

```
<ref>.magic
```
