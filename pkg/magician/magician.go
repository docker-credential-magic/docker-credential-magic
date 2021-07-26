package magician

import (
	"archive/tar"
	"bytes"
	"embed"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

const (
	pathPrefix = "/opt/magic"
)

var (
	helpers = []string{
		"acr-env",
		"ecr-login",
		"gcr",
		"magic", // our custom helper
	}

	//go:embed credential-helpers/*
	embedded embed.FS
)

type (
	MagicOption func(*magicOperation)

	magicOperation struct {
		tag string
	}
)

func MagicOptWithTag(tag string) MagicOption {
	return func(operation *magicOperation) {
		operation.tag = tag
	}
}

func Abracadabra(src string, options ...MagicOption) error {
	operation := &magicOperation{}
	for _, option := range options {
		option(operation)
	}

	var tag string
	if operation.tag != "" {
		tag = operation.tag
	} else {
		tag = src
	}

	dst, err := name.ParseReference(tag)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", tag, err)
	}

	log.Println(fmt.Sprintf("Pulling %s ...", src))

	base, err := crane.Pull(src)
	if err != nil {
		return fmt.Errorf("pulling %q: %v", src, err)
	}

	log.Println("Augmenting image with credential helpers ...")

	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	for _, helper := range helpers {
		filename := fmt.Sprintf("credential-helpers/docker-credential-%s", helper)
		file, err := embedded.Open(filename)
		if err != nil {
			return fmt.Errorf("opening file %q: %v", filename, err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("stat file %q: %v", file, err)
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
			return fmt.Errorf("writing header %q: %v", header, err)
		}

		if _, err := io.Copy(tw, file); err != nil {
			return fmt.Errorf("copy to tar %q: %v", file, err)
		}
	}

	// Create our custom Docker config file, with magic catch-all
	dockerConfigFileRaw := "{\"credsStore\":\"magic\"}\n"
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
		return fmt.Errorf("writing header of json file %q: %v", header, err)
	}
	if _, err := io.Copy(tw, strings.NewReader(dockerConfigFileRaw)); err != nil {
		return fmt.Errorf("copy json file to tar: %v", err)
	}

	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		return fmt.Errorf("layer from reader: %v", err)
	}

	img, err := mutate.AppendLayers(base, newLayer)
	if err != nil {
		return fmt.Errorf("append layers: %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("load image config: %v", err)
	}

	cfg = cfg.DeepCopy()
	updatePath(cfg)
	updateDockerConfig(cfg)

	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		return fmt.Errorf("mutate config file: %v", err)
	}

	log.Println(fmt.Sprintf("Pushing image to %s ...", tag))

	opts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	err = remote.Write(dst, img, opts...)
	if err != nil {
		return fmt.Errorf("remote write: %v", err)
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
