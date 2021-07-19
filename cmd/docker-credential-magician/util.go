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

const (
	pathPrefix = "/opt/magic"
)

var helpers = []string{
	"acr-env",
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
		name := fmt.Sprintf("%s/bin/docker-credential-%s", strings.TrimPrefix(pathPrefix, "/"), helper)
		header := &tar.Header{
			Name:     name,
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

	// Create our custom Docker config file, with magic catch-all
	dockerConfigFileRaw := "{\"credsStore\":\"magic\"}"
	creationTime := v1.Time{}
	name := fmt.Sprintf("%s/config.json", strings.TrimPrefix(pathPrefix, "/"))
	header := &tar.Header{
		Name:     name,
		Size:     int64(len(dockerConfigFileRaw)),
		Typeflag: tar.TypeReg,
		Mode:     0555,
		ModTime:  creationTime.Time,
	}
	if err := tw.WriteHeader(header); err != nil {
		panic(err)
	}
	if _, err := io.Copy(tw, strings.NewReader(dockerConfigFileRaw)); err != nil {
		panic(err)
	}

	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		panic(err)
	}

	img, err := mutate.AppendLayers(base, newLayer)
	if err != nil {
		panic(err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		panic(err)
	}

	cfg = cfg.DeepCopy()
	updatePath(cfg)
	updateDockerConfig(cfg)

	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		panic(err)
	}

	return img
}

// Adapted from https://github.com/google/ko/blob/ab4d264103bd4931c6721d52bfc9d1a2e79c81d1/pkg/build/gobuild.go#L765
func updatePath(cf *v1.ConfigFile) {
	newPath := fmt.Sprintf("%s/bin", pathPrefix)

	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			// Expect environment variables to be in the form KEY=VALUE, so this is unexpected.
			continue
		}
		key, value := parts[0], parts[1]
		if key == "PATH" {
			value = fmt.Sprintf("%s:%s", newPath, value)
			cf.Config.Env[i] = "PATH=" + value
			return
		}
	}

	// If we get here, we never saw PATH.
	cf.Config.Env = append(cf.Config.Env, "PATH="+newPath)
}

func updateDockerConfig(cf *v1.ConfigFile) {
	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if key == "DOCKER_CONFIG" {
			cf.Config.Env[i] = "DOCKER_CONFIG=" + pathPrefix
		}
	}
	cf.Config.Env = append(cf.Config.Env, "DOCKER_CONFIG="+pathPrefix)
}
