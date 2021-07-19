package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	MagicOption func(*magicOperation)

	magicOperation struct{}
)

func Abracadabra(ref string, options ...MagicOption) error {
	operation := &magicOperation{}
	for _, option := range options {
		option(operation)
	}

	newRef := fmt.Sprintf("%s.magic", ref)

	base, err := crane.Pull(ref)
	if err != nil {
		return err
	}

	tag, err := name.NewTag(newRef)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	for _, helper := range helpers {
		file, err := pkger.Open(fmt.Sprintf("/credential-helpers/docker-credential-%s", helper))
		if err != nil {
			return err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return err
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
			return err
		}

		if _, err := io.Copy(tw, file); err != nil {
			return err
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
		return err
	}
	if _, err := io.Copy(tw, strings.NewReader(dockerConfigFileRaw)); err != nil {
		return err
	}

	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		return err
	}

	img, err := mutate.AppendLayers(base, newLayer)
	if err != nil {
		return err
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return err
	}

	cfg = cfg.DeepCopy()
	updatePath(cfg)
	updateDockerConfig(cfg)

	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		return err
	}

	if responseRaw, err := daemon.Write(tag, img); err != nil {
		return err
	} else {
		type daemonResponseLine struct {
			Stream string
		}
		for _, line := range strings.Split(responseRaw, "\n") {
			var lineParsed daemonResponseLine
			if err := json.Unmarshal([]byte(line), &lineParsed); err == nil {
				if stream := lineParsed.Stream; stream != "" {
					log.Println(strings.TrimSuffix(stream, "\n"))
				}
			}
		}
	}

	return nil
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
