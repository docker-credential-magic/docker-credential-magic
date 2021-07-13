package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/markbates/pkger"
)

var helpers = []string{
	"acr-linux",
	"ecr-login",
	"gcr",
	"magic", // our custom helper
}

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
		panic("usage: docker-credential-magician <ref>")
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
	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	for _, helper := range helpers {
		file, err := pkger.Open(fmt.Sprintf("/credential-helpers/docker-credential-%s", helper))
		if err != nil {
			panic(err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			panic(err)
		}

		creationTime := v1.Time{}
		header := &tar.Header{
			Name:     fmt.Sprintf("usr/local/bin/docker-credential-%s", helper),
			Size:     info.Size(),
			Typeflag: tar.TypeReg,
			// Borrowed from: https://github.com/google/ko/blob/ab4d264103bd4931c6721d52bfc9d1a2e79c81d1/pkg/build/gobuild.go#L477
			// Use a fixed Mode, so that this isn't sensitive to the directory and umask
			// under which it was created. Additionally, windows can only set 0222,
			// 0444, or 0666, none of which are executable.
			Mode:    0555,
			ModTime: creationTime.Time,
		}

		if err := tw.WriteHeader(header); err != nil {
			panic(err)
		}

		if _, err := io.Copy(tw, file); err != nil {
			panic(err)
		}
	}

	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		panic(err)
	}

	img, err := mutate.AppendLayers(base, newLayer)
	if err != nil {
		panic(err)
	}

	return img
}
