package main

import (
	"github.com/docker-credential-magic/docker-credential-magic/pkg/magician"
)

func main() {
	src := "gcr.io/projectsigstore/cosign/ci/cosign:v0.5.0"
	dst := "localhost:5000/cosign:v0.5.0-magic"

	err := magician.Mutate(src, magician.MutateOptWithTag(dst))
	if err != nil {
		panic(err)
	}
}