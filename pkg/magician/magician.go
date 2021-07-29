package magician

import (
	"archive/tar"
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"gopkg.in/yaml.v2"

	"github.com/docker-credential-magic/docker-credential-magic/pkg/types"
)

const (
	pathPrefix     = "/opt/magic"
	binarySubdir   = "bin"
	mappingsSubdir = "etc"
)

var (
	//go:embed credential-helpers/* default-mappings/*
	embedded embed.FS
)

type (
	MagicOption func(*magicOperation)

	magicOperation struct {
		tag     string
		helpers []string
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

	// Validate the tag
	dst, err := name.ParseReference(tag)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", tag, err)
	}

	// Validate the requested helpers to include
	supportedHelpers, err := getSupportedHelpers()
	if err != nil {
		return fmt.Errorf("get supported helpers: %v", err)
	}
	var requestedHelpers []string
	if len(operation.helpers) > 0 {
		// Make sure that the requested helpers are valid/supported
		for _, slug := range operation.helpers {
			slugLower := strings.ToLower(slug)
			var isValid bool
			for _, h := range supportedHelpers {
				if slugLower == h {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("unspported helper: %s", slug)
			}
			requestedHelpers = append(requestedHelpers, slugLower)
		}
	} else {
		// If no helpers requested, then default to all
		requestedHelpers = supportedHelpers
	}

	// Attempt to pull the image
	log.Println(fmt.Sprintf("Pulling %s ...", src))
	base, err := crane.Pull(src)
	if err != nil {
		return fmt.Errorf("pulling %q: %v", src, err)
	}

	// Build the tarball containing our new files
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	var helperNames []string
	for _, slug := range requestedHelpers {
		mappingsFilename := fmt.Sprintf("default-mappings/%s.yml", slug)
		helperName, err := writeEmbeddedFileToTarAtPrefix(tw, mappingsFilename, mappingsSubdir)
		if err != nil {
			return fmt.Errorf("write mappings file %s to tar: %v", mappingsFilename, err)
		}
		helperNames = append(helperNames, helperName)
	}

	// Add our magic helper to the mix
	helperNames = append(helperNames, "magic")

	for _, helperName := range helperNames {
		helperFilename := fmt.Sprintf("credential-helpers/docker-credential-%s", helperName)
		_, err = writeEmbeddedFileToTarAtPrefix(tw, helperFilename, binarySubdir)
		if err != nil {
			return fmt.Errorf("write helper file %s to tar: %v", helperFilename, err)
		}
	}

	// Create our custom Docker config file, with magic catch-all
	dockerConfigFileRaw := "{\"credsStore\":\"magic\"}\n"
	creationTime := v1.Time{}
	name := fmt.Sprintf("%s/config.json", strings.TrimPrefix(pathPrefix, "/"))
	log.Printf("Adding /%s ...\n", name)
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

	// Create layer
	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		return fmt.Errorf("layer from reader: %v", err)
	}

	// Append the layer to image
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
	updateMagicMappings(cfg)

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

func writeEmbeddedFileToTarAtPrefix(tw *tar.Writer, filename string, prefix string) (string, error) {
	basename := path.Base(filename)
	tarFilename := fmt.Sprintf("%s/%s/%s",
		strings.TrimPrefix(pathPrefix, "/"), prefix, basename)
	log.Printf("Adding /%s ...\n", tarFilename)
	file, err := embedded.Open(filename)
	if err != nil {
		return "", fmt.Errorf("opening embedded file %s: %v", filename, err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("reader readall file %s: %v", filename, err)
	}

	// In the case of the mappings files, extract the helper name
	var helper string
	if strings.HasSuffix(basename, ".yml") {
		var m types.HelperMapping
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return "", fmt.Errorf("parsing mappings for %s: %v", filename, err)
		}
		helper = m.Helper
	}

	// Copy file into the tar
	creationTime := v1.Time{}
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file %s: %v", filename, err)
	}
	size := info.Size()
	header := &tar.Header{
		Name:     tarFilename,
		Size:     size,
		Typeflag: tar.TypeReg,
		// Borrowed from: https://github.com/google/ko/blob/ab4d264103bd4931c6721d52bfc9d1a2e79c81d1/pkg/build/gobuild.go#L477
		// Use a fixed Mode, so that this isn't sensitive to the directory and umask
		// under which it was created. Additionally, windows can only set 0222,
		// 0444, or 0666, none of which are executable.
		Mode:    0555,
		ModTime: creationTime.Time,
	}
	if err := tw.WriteHeader(header); err != nil {
		return "", fmt.Errorf("writing header %q: %v", header, err)
	}
	if _, err := io.Copy(tw, bytes.NewBuffer(b)); err != nil {
		return "", fmt.Errorf("copy to tar %q: %v", file, err)
	}

	return helper, nil
}

// Adapted from https://github.com/google/ko/blob/ab4d264103bd4931c6721d52bfc9d1a2e79c81d1/pkg/build/gobuild.go#L765
func updatePath(cf *v1.ConfigFile) {
	newPath := fmt.Sprintf("%s/%s", pathPrefix, binarySubdir)

	log.Printf("Prepending PATH with %s ...\n", newPath)

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
	log.Printf("Setting DOCKER_CONFIG to %s ...\n", pathPrefix)
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

func updateMagicMappings(cf *v1.ConfigFile) {
	magicMappings := fmt.Sprintf("%s/%s", pathPrefix, mappingsSubdir)
	log.Printf("Setting DOCKER_CREDENTIAL_MAGIC_CONFIG to %s ...\n", magicMappings)
	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if key == "DOCKER_CREDENTIAL_MAGIC_CONFIG" {
			cf.Config.Env[i] = "DOCKER_CREDENTIAL_MAGIC_CONFIG=" + magicMappings
		}
	}
	cf.Config.Env = append(cf.Config.Env, "DOCKER_CREDENTIAL_MAGIC_CONFIG="+magicMappings)
}

func getSupportedHelpers() ([]string, error) {
	mappings, err := embedded.ReadDir("default-mappings")
	if err != nil {
		return nil, err
	}
	var supportedHelpers []string
	for _, mapping := range mappings {
		filename := path.Base(mapping.Name())
		slug := strings.TrimSuffix(filename, path.Ext(filename))
		supportedHelpers = append(supportedHelpers, slug)
	}
	return supportedHelpers, nil
}
