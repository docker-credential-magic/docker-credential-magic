package magician

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
)

var (
	testCacheRootDir         = "docker-credential-magician-test"
	testHtpasswdFileBasename = "authtest.htpasswd"
	testUsername             = "myuser"
	testPassword             = "mypass"
)

type MutateTestSuite struct {
	suite.Suite
	CacheRootDir       string
	DockerRegistryHost string
	TestReferences     []*name.Reference
	RemoteOpts         []remote.Option
}

func (suite *MutateTestSuite) SetupSuite() {
	suite.CacheRootDir = testCacheRootDir
	os.RemoveAll(suite.CacheRootDir)
	os.Mkdir(suite.CacheRootDir, 0700)

	// create htpasswd file (w BCrypt, which is required)
	pwBytes, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	suite.Nil(err, "no error generating bcrypt password for test htpasswd file")
	htpasswdPath := filepath.Join(suite.CacheRootDir, testHtpasswdFileBasename)
	err = ioutil.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", testUsername, string(pwBytes))), 0644)
	suite.Nil(err, "no error creating test htpasswd file")

	// Registry config
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.Nil(err, "no error finding free port for test registry")
	suite.DockerRegistryHost = fmt.Sprintf("localhost:%d", port)
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": "localhost",
			"path":  htpasswdPath,
		},
	}
	dockerRegistry, err := registry.NewRegistry(context.Background(), config)
	suite.Nil(err, "no error creating test registry")

	// Create the default keychain
	dockerConfigDirAbsPath, err := filepath.Abs(filepath.Join(suite.CacheRootDir, ".docker"))
	suite.Nil(err, "no error getting docker config absolute path")
	os.Mkdir(dockerConfigDirAbsPath, 0700)
	dockerConfigFileContents := []byte(fmt.Sprintf(`{
	"auths": {
		"%s": {
			"username": "%s",
			"password": "%s"
		}
	}
}`, suite.DockerRegistryHost, testUsername, testPassword))
	dockerConfigFileAbsPath := filepath.Join(dockerConfigDirAbsPath, "config.json")
	err = ioutil.WriteFile(dockerConfigFileAbsPath, dockerConfigFileContents, 0644)
	suite.Nil(err, "no error creating docker auth config file")
	os.Setenv("DOCKER_CONFIG", dockerConfigDirAbsPath)
	suite.RemoteOpts = []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	// Build test refs used in individual tests
	var testReferences []*name.Reference
	for i := range []int{0, 1, 2, 3} {
		ref, err := name.ParseReference(fmt.Sprintf("%s/magician:test%d", suite.DockerRegistryHost, i))
		suite.Nil(err, fmt.Sprintf("parsing reference for test%d setup", i))
		testReferences = append(testReferences, &ref)
	}
	suite.TestReferences = testReferences

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}

func (suite *MutateTestSuite) TearDownSuite() {
	os.RemoveAll(suite.CacheRootDir)
}

func (suite *MutateTestSuite) Test_0_HappyPath() {
	img := empty.Image
	ref := *suite.TestReferences[0]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test0 setup")
	refStr := ref.String()

	altTag := fmt.Sprintf("%s.magic", ref.String())
	opts := []MutateOption{
		MutateOptWithTag(altTag),
		MutateOptWithWriter(os.Stdout),
		MutateOptWithUserAgent("gotta-test-this-opt-somewhere"),
	}
	err = Mutate(refStr, opts...)
	suite.Nil(err, "test0 Mutate fails with alt tag")

	filesAlt, envAlt, err := extractImage(altTag)
	suite.Nil(err, "test0 Mutate fails extracting pushed alt tag")

	err = Mutate(refStr)
	suite.Nil(err, "test0 Mutate fails without alt tag")

	filesReg, envReg, err := extractImage(altTag)
	suite.Nil(err, "test0 Mutate fails extracting pushed reg tag")

	for _, files := range [][]string{filesAlt, filesReg} {
		suite.Contains(files, "/opt/magic/etc/aws.yml")
		suite.Contains(files, "/opt/magic/etc/azure.yml")
		suite.Contains(files, "/opt/magic/etc/gcp.yml")
		suite.Contains(files, "/opt/magic/bin/docker-credential-ecr-login")
		suite.Contains(files, "/opt/magic/bin/docker-credential-acr-env")
		suite.Contains(files, "/opt/magic/bin/docker-credential-gcr")
		suite.Contains(files, "/opt/magic/bin/docker-credential-magic")
		suite.Contains(files, "/opt/magic/config.json")
	}

	for _, env := range [][]string{envAlt, envReg} {
		suite.Contains(env, "DOCKER_CONFIG=/opt/magic")
		suite.Contains(env, "DOCKER_CREDENTIAL_MAGIC_CONFIG=/opt/magic")
		var path string
		var dockerOrigConfig string
		for _, v := range env {
			if strings.HasPrefix(v, "PATH=") {
				path = v
			} else if strings.HasPrefix(v, "DOCKER_ORIG_CONFIG=") {
				dockerOrigConfig = v
			}
		}
		suite.NotEqual("", path, "PATH not found")
		suite.True(strings.HasPrefix(path, "PATH=/opt/magic/bin"))
		suite.Equal("", dockerOrigConfig, "DOCKER_ORIG_CONFIG found")
		suite.NotContains(env, "")
	}
}

func (suite *MutateTestSuite) Test_1_ExistingEnvVars() {
	empty := empty.Image
	cfg, err := empty.ConfigFile()
	suite.Nil(err, "test1 extract base image config")

	// Set DOCKER_CONFIG and PATH in the image
	cfg = cfg.DeepCopy()
	cfg.Config.Env = append(cfg.Config.Env, "DOCKER_CONFIG=/whoop/sie/daisies")
	cfg.Config.Env = append(cfg.Config.Env, "PATH=/bloop/bin")

	img, err := mutate.ConfigFile(empty, cfg)
	suite.Nil(err, "test1 set config")

	ref := *suite.TestReferences[1]
	err = remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test1 setup")

	err = Mutate(ref.String())
	suite.Nil(err, "test1 Mutate fails without alt tag")

	_, env, err := extractImage(ref.String())
	suite.Nil(err, "test1 Mutate fails extracting pushed reg tag")

	var path string
	var dockerOrigConfig string
	for _, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			path = v
		} else if strings.HasPrefix(v, "DOCKER_ORIG_CONFIG=") {
			dockerOrigConfig = v
		}
	}
	suite.Equal("PATH=/opt/magic/bin:/bloop/bin", path)
	suite.Equal("DOCKER_ORIG_CONFIG=/whoop/sie/daisies", dockerOrigConfig)
}

func (suite *MutateTestSuite) Test_2_LimitedIncludedHelpers() {
	img := empty.Image
	ref := *suite.TestReferences[2]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test2 setup")

	err = Mutate(ref.String(), MutateOptWithIncludeHelpers([]string{"azure", "gcp"}))
	suite.Nil(err, "test2 Mutate fails without alt tag")

	files, _, err := extractImage(ref.String())
	suite.Nil(err, "test2 Mutate fails extracting pushed reg tag")

	suite.NotContains(files, "/opt/magic/etc/aws.yml")
	suite.Contains(files, "/opt/magic/etc/azure.yml")
	suite.Contains(files, "/opt/magic/etc/gcp.yml")
	suite.NotContains(files, "/opt/magic/bin/docker-credential-ecr-login")
	suite.Contains(files, "/opt/magic/bin/docker-credential-acr-env")
	suite.Contains(files, "/opt/magic/bin/docker-credential-gcr")
	suite.Contains(files, "/opt/magic/bin/docker-credential-magic")
	suite.Contains(files, "/opt/magic/config.json")

	// Unsupported helpers
	err = Mutate(ref.String(), MutateOptWithIncludeHelpers([]string{"boop"}))
	suite.NotNil(err, "test2 Mutate does not fail with invalid include")
}

func (suite *MutateTestSuite) Test_3_CustomHelpersAndMappings() {
	img := empty.Image
	ref := *suite.TestReferences[3]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test2 setup")

	// Missing custom mappings dir
	err = Mutate(ref.String(), MutateOptWithMappingsDir("some/nonexistant/path"))
	suite.NotNil(err, "test2 Mutate does not fail with invalid mappings path")

	// Missing custom helpers dir
	err = Mutate(ref.String(), MutateOptWithHelpersDir("some/nonexistant/path"))
	suite.NotNil(err, "test2 Mutate does not fail with invalid helpers path")

	// Valid
	err = Mutate(ref.String(),
		MutateOptWithMappingsDir("../../testdata/mappings/valid"),
		MutateOptWithHelpersDir("../../testdata/helpers"),
		MutateOptWithIncludeHelpers([]string{"example"}))
	suite.Nil(err, "test2 Mutate fails with valid custom dirs")

	// Invalid (missing fields)
	err = Mutate(ref.String(),
		MutateOptWithMappingsDir("../../testdata/mappings/invalid-missing-fields"),
		MutateOptWithHelpersDir("../../testdata/helpers"),
		MutateOptWithIncludeHelpers([]string{"example"}))
	suite.NotNil(err, "test2 Mutate does not fails with invalid custom dirs (missing fields)")

	// Invalid (bad yaml)
	err = Mutate(ref.String(),
		MutateOptWithMappingsDir("../../testdata/mappings/invalid-bad-yaml"),
		MutateOptWithHelpersDir("../../testdata/helpers"),
		MutateOptWithIncludeHelpers([]string{"example"}))
	suite.NotNil(err, "test2 Mutate does not fails with invalid custom dirs (bad yaml)")
}

func (suite *MutateTestSuite) Test_3_BadInput() {
	badRefStr := fmt.Sprintf("%s/magician:::::woo!", suite.DockerRegistryHost)
	err := Mutate(badRefStr)
	suite.NotNil(err, "no error with bad ref")

	missingRefStr := fmt.Sprintf("%s/magician:aintrealdawg", suite.DockerRegistryHost)
	err = Mutate(missingRefStr)
	suite.NotNil(err, "no error with missing ref")
}

func TestMagicianTestSuite(t *testing.T) {
	suite.Run(t, new(MutateTestSuite))
}

// Pull an image and return the filenames in the final layer and env
func extractImage(ref string) ([]string, []string, error) {
	pulled, err := crane.Pull(ref)
	if err != nil {
		return nil, nil, err
	}
	layers, err := pulled.Layers()
	if err != nil {
		return nil, nil, err
	}
	finalLayer := layers[len(layers) - 1]
	layerReader, err := finalLayer.Uncompressed()
	if err != nil {
		return nil, nil, err
	}
	defer layerReader.Close()
	tarReader := tar.NewReader(layerReader)
	var files []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		files = append(files, fmt.Sprintf("/%s", header.Name))
	}
	cfg, err := pulled.ConfigFile()
	if err != nil {
		return nil, nil, err
	}
	env := cfg.Config.Env
	return files, env, nil
}
