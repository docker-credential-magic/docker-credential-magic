module github.com/docker-credential-magic/docker-credential-magic

go 1.16

replace (
	github.com/docker/cli => github.com/docker/cli v20.10.3-0.20210730192652-7cf5cd6dec98+incompatible
	github.com/docker/docker => github.com/moby/moby v20.10.3-0.20210803165202-52af46671691+incompatible
)

require (
	github.com/adrg/xdg v0.3.3
	github.com/docker/cli v20.10.7+incompatible
	github.com/docker/docker v20.10.7+incompatible
	github.com/google/go-containerregistry v0.6.0
	github.com/spf13/cobra v1.2.1
	gopkg.in/yaml.v2 v2.4.0
)
