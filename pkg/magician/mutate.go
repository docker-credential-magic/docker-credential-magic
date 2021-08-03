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
		src              string
		logger           *log.Logger
		dst              name.Reference
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
func Mutate(src string, options ...MutateOption) error {
	// Create default operation object
	operation := &mutateOperation{
		configurable: &mutateOperationConfigurable{
			writer: ioutil.Discard,
		},
		runtime: &mutateOperationRuntime{
			src: src,
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
		mutateStepSetDst,
		mutateStepSetSupportedHelpers,
		mutateStepSetRequestedHelpers,

		// Attempt to pull the base image
		mutateStepPullBaseImage,

		// Build new image with helpers, mappings, env vars, etc.
		// TODO: break this step into multiple?
		mutateStepBuildNewImage,

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

func mutateStepSetDst(operation *mutateOperation) error {
	ref := operation.runtime.src
	if operation.configurable.tag != "" {
		ref = operation.configurable.tag
	}
	dst, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", ref, err)
	}
	operation.runtime.dst = dst
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
	operation.runtime.logger.Printf("Pulling %s ...\n", operation.runtime.src)
	baseImage, err := crane.Pull(operation.runtime.src)
	if err != nil {
		return fmt.Errorf("pulling %q: %v", operation.runtime.src, err)
	}
	operation.runtime.baseImage = baseImage
	return nil
}

func mutateStepBuildNewImage(operation *mutateOperation) error {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	var helperNames []string
	for _, slug := range operation.runtime.requestedHelpers {
		mappingsFilename := filepath.Join(constants.EmbeddedParentDir,
			fmt.Sprintf("%s.%s", slug, constants.ExtensionYAML))
		helperName, err := writeEmbeddedFileToTarAtPrefix(operation.runtime.logger, tw,
			mappingsFilename, operation.configurable.mappingsDir, operation.configurable.helpersDir, constants.MappingsSubdir)
		if err != nil {
			return fmt.Errorf("write mappings file %s to tar: %v", mappingsFilename, err)
		}
		helperNames = append(helperNames, helperName)
	}
	helperNames = append(helperNames, "magic")
	for _, helperName := range helperNames {
		helperFilename := filepath.Join(constants.EmbeddedParentDir,
			fmt.Sprintf("docker-credential-%s", helperName))
		_, err := writeEmbeddedFileToTarAtPrefix(operation.runtime.logger, tw,
			helperFilename, operation.configurable.mappingsDir, operation.configurable.helpersDir, constants.BinariesSubdir)
		if err != nil {
			return fmt.Errorf("write helper file %s to tar: %v", helperFilename, err)
		}
	}
	creationTime := v1.Time{}
	name := fmt.Sprintf("%s/%s", strings.TrimPrefix(constants.MagicRootDir, "/"), constants.DockerConfigFileBasename)
	operation.runtime.logger.Printf("Adding /%s ...\n", name)
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
	newLayer, err := tarball.LayerFromReader(&b)
	if err != nil {
		return fmt.Errorf("layer from reader: %v", err)
	}
	img, err := mutate.AppendLayers(operation.runtime.baseImage, newLayer)
	if err != nil {
		return fmt.Errorf("append layers: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("load image config: %v", err)
	}
	cfg = cfg.DeepCopy()
	updatePath(operation.runtime.logger, cfg)
	updateDockerConfig(operation.runtime.logger, cfg)
	updateMagicMappings(operation.runtime.logger, cfg)
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		return fmt.Errorf("mutate config file: %v", err)
	}
	operation.runtime.newImage = img
	return nil
}

func mutateStepPushNewImage(operation *mutateOperation) error {
	operation.runtime.logger.Printf("Pushing image to %s ...\n",
		operation.runtime.dst.String())
	opts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}
	if operation.configurable.userAgent != "" {
		opts = append(opts, remote.WithUserAgent(operation.configurable.userAgent))
	}
	if err := remote.Write(operation.runtime.dst,
		operation.runtime.newImage, opts...); err != nil {
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
