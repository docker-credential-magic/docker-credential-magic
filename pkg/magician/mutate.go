package magician

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"gopkg.in/yaml.v2"

	"github.com/docker-credential-magic/docker-credential-magic/internal/constants"
	"github.com/docker-credential-magic/docker-credential-magic/internal/embedded/helpers"
	"github.com/docker-credential-magic/docker-credential-magic/internal/embedded/mappings"
	"github.com/docker-credential-magic/docker-credential-magic/internal/types"
)

type (
	MutateOption func(*mutateOperation)

	mutateOperation struct {
		tag            string
		userAgent      string
		helpersDir     string
		mappingsDir    string
		includeHelpers []string
		writer         io.Writer
	}
)

func MutateOptWithTag(tag string) MutateOption {
	return func(operation *mutateOperation) {
		operation.tag = tag
	}
}

func MutateOptWithUserAgent(userAgent string) MutateOption {
	return func(operation *mutateOperation) {
		operation.userAgent = userAgent
	}
}

func MutateOptWithHelpersDir(helpersDir string) MutateOption {
	return func(operation *mutateOperation) {
		operation.helpersDir = helpersDir
	}
}

func MutateOptWithMappingsDir(mappingsDir string) MutateOption {
	return func(operation *mutateOperation) {
		operation.mappingsDir = mappingsDir
	}
}

func MutateOptWithIncludeHelpers(includeHelpers []string) MutateOption {
	return func(operation *mutateOperation) {
		operation.includeHelpers = includeHelpers
	}
}

func MutateOptWithWriter(writer io.Writer) MutateOption {
	return func(operation *mutateOperation) {
		operation.writer = writer
	}
}

func Mutate(src string, options ...MutateOption) error {
	operation := &mutateOperation{
		writer: ioutil.Discard,
	}
	for _, option := range options {
		option(operation)
	}
	logger := log.Default()
	logger.SetOutput(operation.writer)

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
	supportedHelpers, err := getSupportedHelpers(operation.mappingsDir)
	if err != nil {
		return fmt.Errorf("get supported helpers: %v", err)
	}
	var requestedHelpers []string
	if len(operation.includeHelpers) > 0 {
		// Make sure that the requested helpers are valid/supported
		for _, slug := range operation.includeHelpers {
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
	logger.Printf("Pulling %s ...\n", src)
	base, err := crane.Pull(src)
	if err != nil {
		return fmt.Errorf("pulling %q: %v", src, err)
	}

	// Build the tarball containing our new files
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	var helperNames []string
	for _, slug := range requestedHelpers {
		mappingsFilename := filepath.Join(constants.EmbeddedParentDir,
			fmt.Sprintf("%s.%s", slug, constants.ExtensionYAML))
		helperName, err := writeEmbeddedFileToTarAtPrefix(logger, tw,
			mappingsFilename, operation.mappingsDir, operation.helpersDir, constants.MappingsSubdir)
		if err != nil {
			return fmt.Errorf("write mappings file %s to tar: %v", mappingsFilename, err)
		}
		helperNames = append(helperNames, helperName)
	}

	// Add our magic helper to the mix
	helperNames = append(helperNames, "magic")

	for _, helperName := range helperNames {
		helperFilename := filepath.Join(constants.EmbeddedParentDir,
			fmt.Sprintf("docker-credential-%s", helperName))
		_, err = writeEmbeddedFileToTarAtPrefix(logger, tw,
			helperFilename, operation.mappingsDir, operation.helpersDir, constants.BinariesSubdir)
		if err != nil {
			return fmt.Errorf("write helper file %s to tar: %v", helperFilename, err)
		}
	}

	// Create our custom Docker config file, with magic catch-all
	creationTime := v1.Time{}
	name := fmt.Sprintf("%s/%s", strings.TrimPrefix(constants.MagicRootDir, "/"), constants.DockerConfigFileBasename)
	logger.Printf("Adding /%s ...\n", name)
	header := &tar.Header{
		Name:     name,
		Size:     int64(len(constants.DockerConfigFileContents)),
		Typeflag: tar.TypeReg,
		Mode:     0555,
		ModTime:  creationTime.Time,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing header of json file %q: %v", header, err)
	}
	if _, err := io.Copy(tw, strings.NewReader(constants.DockerConfigFileContents)); err != nil {
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
	updatePath(logger, cfg)
	updateDockerConfig(logger, cfg)
	updateMagicMappings(logger, cfg)

	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		return fmt.Errorf("mutate config file: %v", err)
	}

	logger.Printf("Pushing image to %s ...\n", tag)

	opts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	if operation.userAgent != "" {
		opts = append(opts, remote.WithUserAgent(operation.userAgent))
	}

	err = remote.Write(dst, img, opts...)
	if err != nil {
		return fmt.Errorf("remote write: %v", err)
	}

	return nil
}

// TODO: clean this up / break this up big time...
func writeEmbeddedFileToTarAtPrefix(logger *log.Logger, tw *tar.Writer, filename string,
	mappingsDir string, helpersDir string, prefix string) (string, error) {
	basename := path.Base(filename)
	tarFilename := fmt.Sprintf("%s/%s/%s",
		strings.TrimPrefix(constants.MagicRootDir, "/"), prefix, basename)
	logger.Printf("Adding /%s ...\n", tarFilename)
	var file fs.File
	var err error
	if strings.HasSuffix(filename, fmt.Sprintf(".%s", constants.ExtensionYAML)) {
		if mappingsDir == "" {
			file, err = mappings.Embedded.Open(filename)
		} else {
			newPath := filepath.Join(mappingsDir, basename)
			file, err = os.Open(newPath)
		}
	} else {
		// special case for "docker-credential-magic", always take from embedded
		if helpersDir == "" || basename == "docker-credential-magic" {
			file, err = helpers.Embedded.Open(filename)
		} else {
			newPath := filepath.Join(helpersDir, basename)
			file, err = os.Open(newPath)
		}
	}
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
	if strings.HasSuffix(basename, fmt.Sprintf(".%s", constants.ExtensionYAML)) {
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
func updatePath(logger *log.Logger, cf *v1.ConfigFile) {
	newPath := fmt.Sprintf("%s/%s", constants.MagicRootDir, constants.BinariesSubdir)

	logger.Printf("Prepending %s with %s ...\n", constants.EnvVarPath, newPath)

	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			// Expect environment variables to be in the form KEY=VALUE, so this is unexpected.
			continue
		}
		key, value := parts[0], parts[1]
		if key == constants.EnvVarPath {
			value = fmt.Sprintf("%s:%s", newPath, value)
			cf.Config.Env[i] = constants.EnvVarPath + "=" + value
			return
		}
	}

	// If we get here, we never saw PATH.
	cf.Config.Env = append(cf.Config.Env, constants.EnvVarPath+"="+newPath)
}

func updateDockerConfig(logger *log.Logger, cf *v1.ConfigFile) {
	logger.Printf("Setting %s to %s ...\n", constants.EnvVarDockerConfig, constants.MagicRootDir)
	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if key == constants.EnvVarDockerConfig {
			cf.Config.Env[i] = constants.EnvVarDockerConfig + "=" + constants.MagicRootDir
		}
	}
	cf.Config.Env = append(cf.Config.Env, constants.EnvVarDockerConfig+"="+constants.MagicRootDir)
}

func updateMagicMappings(logger *log.Logger, cf *v1.ConfigFile) {
	logger.Printf("Setting %s to %s ...\n", constants.EnvVarDockerCredentialMagicConfig, constants.MagicRootDir)
	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if key == constants.EnvVarDockerCredentialMagicConfig {
			cf.Config.Env[i] = constants.EnvVarDockerCredentialMagicConfig + "=" + constants.MagicRootDir
		}
	}
	cf.Config.Env = append(cf.Config.Env, constants.EnvVarDockerCredentialMagicConfig+"="+constants.MagicRootDir)
}

func getSupportedHelpers(mappingsDir string) ([]string, error) {
	var entries []fs.DirEntry
	var err error
	if mappingsDir == "" {
		entries, err = mappings.Embedded.ReadDir(constants.EmbeddedParentDir)
	} else {
		entries, err = os.ReadDir(mappingsDir)
	}
	if err != nil {
		return nil, err
	}
	var supportedHelpers []string
	for _, entry := range entries {
		filename := path.Base(entry.Name())
		slug := strings.TrimSuffix(filename, path.Ext(filename))
		supportedHelpers = append(supportedHelpers, slug)
	}
	return supportedHelpers, nil
}
