module github.com/docker-credential-magic/docker-credential-magic

go 1.16

replace github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d

require (
	github.com/adrg/xdg v0.3.3
	github.com/docker/cli v20.10.7+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.7+incompatible
	github.com/google/go-containerregistry v0.6.0
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	gopkg.in/yaml.v2 v2.4.0
)
