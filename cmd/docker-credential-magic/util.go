package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

type (
	config struct {
		OrigRef string
		NewRef  string
	}

	daemonResponseLine struct {
		Stream string
	}
)

func parseConfig() config {
	if len(os.Args) < 2 {
		panic("usage: docker-credential-magic <ref>")
	}
	origRef := os.Args[1]
	return config{
		OrigRef: origRef,
		NewRef:  fmt.Sprintf("%s.magic", origRef),
	}
}

func pullBaseImage(ref string) v1.Image {
	base, err := crane.Pull(ref)
	if err != nil {
		panic(err)
	}
	return base
}

func createTag(ref string) name.Tag {
	tag, err := name.NewTag(ref)
	if err != nil {
		panic(err)
	}
	return tag
}

func pushImageToLocalDaemon(tag name.Tag, base v1.Image) {
	if responseRaw, err := daemon.Write(tag, base); err != nil {
		panic(err)
	} else {
		for _, line := range strings.Split(responseRaw, "\n") {
			var lineParsed daemonResponseLine
			if err := json.Unmarshal([]byte(line), &lineParsed); err == nil {
				if stream := lineParsed.Stream; stream != "" {
					fmt.Printf(stream)
				}
			}
		}
	}
}

func appendCredentialHelpers(base v1.Image) v1.Image {
	b, err := ioutil.ReadFile("credential-helpers/docker-credential-gcr")
	if err != nil {
		panic(err)
	}

	newLayerMap := map[string][]byte{
		"usr/local/bin/docker-credential-gcr": b,
	}
	newLayer, err := crane.Layer(newLayerMap)
	if err != nil {
		panic(err)
	}

	img, err := mutate.AppendLayers(base, newLayer)
	if err != nil {
		panic(err)
	}

	return img
}
