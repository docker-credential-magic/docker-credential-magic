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
	// MutateOption allows setting various configuration settings on a mutate operation.
	MutateOption func(*mutateOperation)

	mutateOperation struct {
		configurable *mutateOperationConfigurable
		runtime      *mutateOperationRuntime
	}

	// The following fields are configurable via options
	mutateOperationConfigurable struct {
		tag            string
		userAgent      string
		helpersDir     string
		mappingsDir    string
		includeHelpers []string
		writer         io.Writer
	}

	// The following fields are *not* configurable via options,
	// and are used to pass data between mutate steps
	mutateOperationRuntime struct {
		source           string
		logger           *log.Logger
		destination      name.Reference
		supportedHelpers []string
		requestedHelpers []string
		baseImage        v1.Image
		newImage         v1.Image
	}

	mutateStep func(o *mutateOperation) error
)

// MutateOptWithTag sets a custom tag to use for a mutate operation.
func MutateOptWithTag(tag string) MutateOption {
	return func(operation *mutateOperation) {
		operation.configurable.tag = tag
	}
}

// MutateOptWithUserAgent sets a custom user agent to use for a mutate operation.
func MutateOptWithUserAgent(userAgent string) MutateOption {
	return func(operation *mutateOperation) {
		operation.configurable.userAgent = userAgent
	}
}

// MutateOptWithHelpersDir sets a custom directory to source helper binaries for a mutate operation.
func MutateOptWithHelpersDir(helpersDir string) MutateOption {
	return func(operation *mutateOperation) {
		operation.configurable.helpersDir = helpersDir
	}
}

// MutateOptWithMappingsDir sets a custom directory to source mappings files for a mutate operation.
func MutateOptWithMappingsDir(mappingsDir string) MutateOption {
	return func(operation *mutateOperation) {
		operation.configurable.mappingsDir = mappingsDir
	}
}

// MutateOptWithIncludeHelpers sets a list of helpers to include in the new image for a mutate operation.
func MutateOptWithIncludeHelpers(includeHelpers []string) MutateOption {
	return func(operation *mutateOperation) {
		operation.configurable.includeHelpers = includeHelpers
	}
}

// MutateOptWithWriter sets an output writer to use for a mutate operation.
func MutateOptWithWriter(writer io.Writer) MutateOption {
	return func(operation *mutateOperation) {
		operation.configurable.writer = writer
	}
}

// Mutate takes a remote source image, builds a new image with various
// Docker credential helpers baked-in, then pushes it back to the registry
// (at the same reference unless otherwise specified with MutateOptWithTag).
func Mutate(source string, options ...MutateOption) error {
	// Create default operation object
	operation := &mutateOperation{
		configurable: &mutateOperationConfigurable{
			writer: ioutil.Discard,
		},
		runtime: &mutateOperationRuntime{
			source: source,
		},
	}

	// Process user-provided options which modify configurable fields
	for _, option := range options {
		option(operation)
	}

	// Run each of the mutate steps in order
	for _, step := range []mutateStep{

		// Prepopulate various runtime fields on the operation
		mutateStepSetLogger,
		mutateStepSetDestination,
		mutateStepSetSupportedHelpers,
		mutateStepSetRequestedHelpers,

		// Attempt to pull the base image
		mutateStepPullBaseImage,

		// Build new image with helpers, mappings, env vars, etc.
		mutateStepAppendImageLayer,
		mutateStepUpdateImageConfig,

		// Push the new image to remote
		mutateStepPushNewImage,
	} {
		if err := step(operation); err != nil {
			return err
		}
	}

	operation.runtime.logger.Println("Done.")
	return nil
}

func mutateStepSetLogger(operation *mutateOperation) error {
	logger := log.Default()
	logger.SetOutput(operation.configurable.writer)
	operation.runtime.logger = logger
	return nil
}

func mutateStepSetDestination(operation *mutateOperation) error {
	ref := operation.runtime.source
	if operation.configurable.tag != "" {
		ref = operation.configurable.tag
	}
	destination, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", ref, err)
	}
	operation.runtime.destination = destination
	return nil
}

func mutateStepSetSupportedHelpers(operation *mutateOperation) error {
	var entries []fs.DirEntry
	var err error
	if operation.configurable.mappingsDir == "" {
		entries, err = mappings.Embedded.ReadDir(constants.EmbeddedParentDir)
	} else {
		entries, err = os.ReadDir(operation.configurable.mappingsDir)
	}
	if err != nil {
		return fmt.Errorf("get supported helpers: %v", err)
	}
	var supportedHelpers []string
	for _, entry := range entries {
		filename := path.Base(entry.Name())
		slug := strings.TrimSuffix(filename, path.Ext(filename))
		supportedHelpers = append(supportedHelpers, slug)
	}
	operation.runtime.supportedHelpers = supportedHelpers
	return nil
}

func mutateStepSetRequestedHelpers(operation *mutateOperation) error {
	var requestedHelpers []string
	if len(operation.configurable.includeHelpers) > 0 {
		// Make sure that the requested helpers are valid/supported
		for _, slug := range operation.configurable.includeHelpers {
			slugLower := strings.ToLower(slug)
			var isValid bool
			for _, h := range operation.runtime.supportedHelpers {
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
		requestedHelpers = operation.runtime.supportedHelpers
	}
	operation.runtime.requestedHelpers = requestedHelpers
	return nil
}

func mutateStepPullBaseImage(operation *mutateOperation) error {
	operation.runtime.logger.Printf("Pulling %s ...\n", operation.runtime.source)
	baseImage, err := crane.Pull(operation.runtime.source)
	if err != nil {
		return fmt.Errorf("pulling %q: %v", operation.runtime.source, err)
	}
	operation.runtime.baseImage = baseImage
	return nil
}

func mutateStepAppendImageLayer(operation *mutateOperation) error {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	// Add the mappings files to tar, extracting the helper names as we go
	var helperNames []string
	for _, slug := range operation.runtime.requestedHelpers {
		embeddedFilename, tarFilename := mutateUtilGetMappingsFilenamesBySlug(slug)
		operation.runtime.logger.Printf("Adding /%s ...\n", tarFilename)
		helperName, err := mutateUtilWriteEmbeddedFileToTar(embeddedFilename, tarFilename, tw, true,
			operation.configurable.mappingsDir, operation.configurable.helpersDir)
		if err != nil {
			return fmt.Errorf("write mappings file %s to tar: %v", embeddedFilename, err)
		}
		helperNames = append(helperNames, helperName)
	}

	// Add our magic helper to the list of helpers for the next step
	helperNames = append(helperNames, constants.MagicCredentialSuffix)

	// Add the helper binaries to tar
	for _, helperName := range helperNames {
		embeddedFilename, tarFilename := mutateUtilGetHelperFilenamesByName(helperName)
		operation.runtime.logger.Printf("Adding /%s ...\n", tarFilename)
		_, err := mutateUtilWriteEmbeddedFileToTar(embeddedFilename, tarFilename, tw, false,
			operation.configurable.mappingsDir, operation.configurable.helpersDir)
		if err != nil {
			return fmt.Errorf("write helper file %s to tar: %v", embeddedFilename, err)
		}
	}

	// Add our custom Docker config.json to tar
	name := fmt.Sprintf("%s/%s", strings.TrimPrefix(constants.MagicRootDir, "/"),
		constants.DockerConfigFileBasename)
	operation.runtime.logger.Printf("Adding /%s ...\n", name)
	err := mutateUtilWriteFileToTar(name, int64(len(constants.DockerConfigFileContents)),
		strings.NewReader(constants.DockerConfigFileContents), tw)
	if err != nil {
		return err
	}

	// Create and append new layer from tarball
	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		return fmt.Errorf("layer from reader: %v", err)
	}
	img, err := mutate.AppendLayers(operation.runtime.baseImage, newLayer)
	if err != nil {
		return fmt.Errorf("append layers: %v", err)
	}
	operation.runtime.newImage = img

	return nil
}

func mutateStepUpdateImageConfig(operation *mutateOperation) error {
	cfg, err := operation.runtime.newImage.ConfigFile()
	if err != nil {
		return fmt.Errorf("load image config: %v", err)
	}
	cfg = cfg.DeepCopy()

	// $PATH
	newPath := fmt.Sprintf("%s/%s", constants.MagicRootDir, constants.BinariesSubdir)
	operation.runtime.logger.Printf("Prepending %s with %s ...\n", constants.EnvVarPath, newPath)
	_, existingPath := mutateUtilGetImageConfigEnvVar(cfg, constants.EnvVarPath)
	if existingPath != "" {
		newPath = fmt.Sprintf("%s:%s", newPath, existingPath)
	}
	mutateUtilSetImageConfigEnvVar(cfg, constants.EnvVarPath, newPath)

	// $DOCKER_ORIG_CONFIG
	// (If an image already has $DOCKER_CONFIG, set this var so we can fallback on it)
	_, existingDockerConfig := mutateUtilGetImageConfigEnvVar(cfg, constants.EnvVarDockerConfig)
	if existingDockerConfig != "" {
		operation.runtime.logger.Printf("Existing %s detected (%s), setting %s ...\n",
			constants.EnvVarDockerConfig, existingDockerConfig, constants.EnvVarDockerOrigConfig)
		mutateUtilSetImageConfigEnvVar(cfg, constants.EnvVarDockerOrigConfig, existingDockerConfig)
	}

	// $DOCKER_CONFIG
	operation.runtime.logger.Printf("Setting %s to %s ...\n",
		constants.EnvVarDockerConfig, constants.MagicRootDir)
	mutateUtilSetImageConfigEnvVar(cfg, constants.EnvVarDockerConfig, constants.MagicRootDir)

	// $DOCKER_CREDENTIAL_MAGIC_CONFIG
	operation.runtime.logger.Printf("Setting %s to %s ...\n",
		constants.EnvVarDockerCredentialMagicConfig, constants.MagicRootDir)
	mutateUtilSetImageConfigEnvVar(cfg, constants.EnvVarDockerCredentialMagicConfig, constants.MagicRootDir)

	operation.runtime.newImage, err = mutate.ConfigFile(operation.runtime.newImage, cfg)
	if err != nil {
		return fmt.Errorf("mutate config file: %v", err)
	}
	return nil
}

func mutateStepPushNewImage(operation *mutateOperation) error {
	operation.runtime.logger.Printf("Pushing image to %s ...\n",
		operation.runtime.destination.String())
	opts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}
	if operation.configurable.userAgent != "" {
		opts = append(opts, remote.WithUserAgent(operation.configurable.userAgent))
	}
	if err := remote.Write(operation.runtime.destination,
		operation.runtime.newImage, opts...); err != nil {
		return fmt.Errorf("remote write: %v", err)
	}
	return nil
}

func mutateUtilGetMappingsFilenamesBySlug(slug string) (string, string) {
	embeddedFilename := filepath.Join(constants.EmbeddedParentDir,
		fmt.Sprintf("%s.%s", slug, constants.ExtensionYAML))
	tarFilename := fmt.Sprintf("%s/%s/%s",
		strings.TrimPrefix(constants.MagicRootDir, "/"),
		constants.MappingsSubdir, path.Base(embeddedFilename))
	return embeddedFilename, tarFilename
}

func mutateUtilGetHelperFilenamesByName(name string) (string, string) {
	embeddedFilename := filepath.Join(constants.EmbeddedParentDir,
		fmt.Sprintf("%s-%s", constants.DockerCredentialPrefix, name))
	tarFilename := fmt.Sprintf("%s/%s/%s",
		strings.TrimPrefix(constants.MagicRootDir, "/"),
		constants.BinariesSubdir, path.Base(embeddedFilename))
	return embeddedFilename, tarFilename
}

// Grab embedded file by path "embeddedFilename" and add to the tar at "tarFilename".
// If "mappingsDir" / "helpersDir" is provided, grab it from there instead.
// If "isMapping" is true, then assume mappings file and attempt to extract helper name.
func mutateUtilWriteEmbeddedFileToTar(embeddedFilename string, tarFilename string,
	tw *tar.Writer, isMapping bool, mappingsDir string, helpersDir string) (string, error) {
	basename := path.Base(embeddedFilename)
	var file fs.File
	var err error
	if isMapping {
		if mappingsDir == "" {
			file, err = mappings.Embedded.Open(embeddedFilename)
		} else {
			newPath := filepath.Join(mappingsDir, basename)
			file, err = os.Open(newPath)
		}
	} else {
		// special case for "docker-credential-magic", always take from embedded
		if helpersDir == "" || basename == fmt.Sprintf("%s-%s",
			constants.DockerCredentialPrefix, constants.MagicCredentialSuffix) {
			fmt.Println("HERE 1")
			file, err = helpers.Embedded.Open(embeddedFilename)
		} else {
			newPath := filepath.Join(helpersDir, basename)
			file, err = os.Open(newPath)
		}
	}
	if err != nil {
		return "", fmt.Errorf("opening embedded file %s: %v", basename, err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("reader readall file %s: %v", basename, err)
	}

	// In the case of the mappings files, extract the helper name
	var helper string
	if isMapping {
		var m types.HelperMapping
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return "", fmt.Errorf("parsing mappings for %s: %v", basename, err)
		}
		helper = m.Helper
	}

	// Copy file into the tar
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file %s: %v", basename, err)
	}
	size := info.Size()
	if err := mutateUtilWriteFileToTar(tarFilename, size, bytes.NewBuffer(b), tw); err != nil {
		return "", err
	}

	return helper, nil
}

func mutateUtilWriteFileToTar(filename string, size int64, reader io.Reader, tw *tar.Writer) error {
	creationTime := v1.Time{}
	header := &tar.Header{
		Name:     filename,
		Size:     size,
		Typeflag: tar.TypeReg,
		// Borrowed from:
		// https://github.com/google/ko/blob/ab4d264103bd4931c6721d52bfc9d1a2e79c81d1/pkg/build/gobuild.go#L477
		// Use a fixed Mode, so that this isn't sensitive to the directory and umask
		// under which it was created. Additionally, windows can only set 0222,
		// 0444, or 0666, none of which are executable.
		Mode:     0555,
		ModTime:  creationTime.Time,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing header %q: %v", header, err)
	}
	if _, err := io.Copy(tw, reader); err != nil {
		return fmt.Errorf("copy to tar %s: %v", filename, err)
	}
	return nil
}

// Adapted from: https://github.com/google/ko/blob/ab4d264103bd4931c6721d52bfc9d1a2e79c81d1/pkg/build/gobuild.go#L765
func mutateUtilGetImageConfigEnvVar(cf *v1.ConfigFile, key string) (int, string) {
	for i, env := range cf.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			// Expect environment variables to be in the form KEY=VALUE, so this is unexpected.
			continue
		}
		k, v := parts[0], parts[1]
		if k == key {
			return i, v
		}
	}
	return -1, ""
}

func mutateUtilSetImageConfigEnvVar(cf *v1.ConfigFile, key string, val string) {
	i, _ := mutateUtilGetImageConfigEnvVar(cf, key)
	if i >= 0 {
		cf.Config.Env[i] = fmt.Sprintf("%s=%s", key, val)
	} else {
		cf.Config.Env = append(cf.Config.Env, fmt.Sprintf("%s=%s", key, val))
	}
}
