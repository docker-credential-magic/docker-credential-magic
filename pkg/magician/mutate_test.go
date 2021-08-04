package magician

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"

	"github.com/docker-credential-magic/docker-credential-magic/internal/constants"
	"github.com/docker-credential-magic/docker-credential-magic/internal/embedded/mappings"
	"github.com/docker-credential-magic/docker-credential-magic/internal/types"
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
	SlugHelperMap      map[string]string
}

func (suite *MutateTestSuite) SetupSuite() {
	slugHelperMap, err := getSlugHelperMap()
	suite.Nil(err, "no error loading slug helper map")
	suite.SlugHelperMap = slugHelperMap

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
	config.HTTP.Addr = fmt.Sprintf("127.0.0.1:%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	config.Auth = configuration.Auth{
		"htpasswd": configuration.Parameters{
			"realm": "localhost",
			"path":  htpasswdPath,
		},
	}

	// If you want to see registry output, comment out these 2 lines
	config.Log.Level = "fatal"
	config.Log.AccessLog.Disabled = true

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
		MutateOptWithWriter(ioutil.Discard), // set to "os.Stdout" to see mutate output
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
		for slug, helper := range suite.SlugHelperMap {
			mappingFilename := fmt.Sprintf("%s/%s/%s.%s",
				constants.MagicRootDir, constants.MappingsSubdir,
				slug, constants.ExtensionYAML)
			suite.Contains(files, mappingFilename)

			helperFilename := fmt.Sprintf("%s/%s/%s-%s",
				constants.MagicRootDir, constants.BinariesSubdir,
				constants.DockerCredentialPrefix, helper)
			suite.Contains(files, helperFilename)
		}

		magicHelperFilename := fmt.Sprintf("%s/%s/%s-%s",
			constants.MagicRootDir, constants.BinariesSubdir,
			constants.DockerCredentialPrefix, constants.MagicCredentialSuffix)
		suite.Contains(files, magicHelperFilename)

		magicConfigFilename := fmt.Sprintf("%s/%s",
			constants.MagicRootDir, constants.DockerConfigFileBasename)
		suite.Contains(files, magicConfigFilename)
	}

	for _, env := range [][]string{envAlt, envReg} {
		suite.Contains(env, fmt.Sprintf("%s=%s",
			constants.EnvVarDockerConfig, constants.MagicRootDir))
		suite.Contains(env, fmt.Sprintf("%s=%s",
			constants.EnvVarDockerCredentialMagicConfig, constants.MagicRootDir))
		var path string
		var dockerOrigConfig string
		for _, v := range env {
			if strings.HasPrefix(v, fmt.Sprintf("%s=", constants.EnvVarPath)) {
				path = v
			} else if strings.HasPrefix(v,
				fmt.Sprintf("%s=", constants.EnvVarDockerOrigConfig)) {
				dockerOrigConfig = v
			}
		}
		suite.NotEqual("", path, fmt.Sprintf("%s not found", constants.EnvVarPath))
		suite.True(strings.HasPrefix(path, fmt.Sprintf("%s=%s/%s",
			constants.EnvVarPath, constants.MagicRootDir, constants.BinariesSubdir)))
		suite.Equal("", dockerOrigConfig, fmt.Sprintf("%s found",
			constants.EnvVarDockerOrigConfig))
		suite.NotContains(env, "")
	}
}

func (suite *MutateTestSuite) Test_1_ExistingEnvVars() {
	empty := empty.Image
	cfg, err := empty.ConfigFile()
	suite.Nil(err, "test1 extract base image config")

	// Set DOCKER_CONFIG and PATH in the image
	cfg = cfg.DeepCopy()
	cfg.Config.Env = append(cfg.Config.Env,
		fmt.Sprintf("%s=/whoop/sie/daisies", constants.EnvVarDockerConfig))
	cfg.Config.Env = append(cfg.Config.Env,
		fmt.Sprintf("%s=/bloop/bin", constants.EnvVarPath))

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
		if strings.HasPrefix(v, fmt.Sprintf("%s=", constants.EnvVarPath)) {
			path = v
		} else if strings.HasPrefix(v,
			fmt.Sprintf("%s=", constants.EnvVarDockerOrigConfig)) {
			dockerOrigConfig = v
		}
	}
	suite.Equal(
		fmt.Sprintf("%s=%s/%s:/bloop/bin", constants.EnvVarPath,
			constants.MagicRootDir, constants.BinariesSubdir), path)
	suite.Equal(
		fmt.Sprintf("%s=/whoop/sie/daisies", constants.EnvVarDockerOrigConfig),
		dockerOrigConfig)
}

func (suite *MutateTestSuite) Test_2_LimitedIncludedHelpers() {
	img := empty.Image
	ref := *suite.TestReferences[2]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test2 setup")

	// Only include the first two slugs in the map
	var firstTwoKeys []string
	for k, _ := range suite.SlugHelperMap {
		firstTwoKeys = append(firstTwoKeys, k)
		if len(firstTwoKeys) == 2 {
			break
		}
	}

	err = Mutate(ref.String(), MutateOptWithIncludeHelpers(firstTwoKeys))
	suite.Nil(err, "test2 Mutate fails without alt tag")

	files, _, err := extractImage(ref.String())
	suite.Nil(err, "test2 Mutate fails extracting pushed reg tag")

	for slug, helper := range suite.SlugHelperMap {
		mappingFilename := fmt.Sprintf("%s/%s/%s.%s",
			constants.MagicRootDir, constants.MappingsSubdir,
			slug, constants.ExtensionYAML)

		helperFilename := fmt.Sprintf("%s/%s/%s-%s",
			constants.MagicRootDir, constants.BinariesSubdir,
			constants.DockerCredentialPrefix, helper)

		var shouldContain bool
		for _, v := range firstTwoKeys {
			if slug == v {
				shouldContain = true
				break
			}
		}

		if shouldContain {
			suite.Contains(files, mappingFilename)
			suite.Contains(files, helperFilename)
		} else {
			suite.NotContains(files, mappingFilename)
			suite.NotContains(files, helperFilename)
		}
	}

	magicHelperFilename := fmt.Sprintf("%s/%s/%s-%s",
		constants.MagicRootDir, constants.BinariesSubdir,
		constants.DockerCredentialPrefix, constants.MagicCredentialSuffix)
	suite.Contains(files, magicHelperFilename)

	magicConfigFilename := fmt.Sprintf("%s/%s",
		constants.MagicRootDir, constants.DockerConfigFileBasename)
	suite.Contains(files, magicConfigFilename)

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

func TestMutateTestSuite(t *testing.T) {
	suite.Run(t, new(MutateTestSuite))
}

// Dynamically load the list of supported helpers
func getSlugHelperMap() (map[string]string, error) {
	slugHelperMap := map[string]string{}
	entries, err := mappings.Embedded.ReadDir(constants.EmbeddedParentDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		filename := path.Base(entry.Name())
		embeddedFilename := filepath.Join(constants.EmbeddedParentDir, filename)
		slug := strings.TrimSuffix(filename, path.Ext(filename))
		file, err := mappings.Embedded.Open(embeddedFilename)
		if err != nil {
			return nil, err
		}
		b, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		var m types.HelperMapping
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return nil, err
		}
		slugHelperMap[slug] = m.Helper
	}
	return slugHelperMap, nil
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
	finalLayer := layers[len(layers)-1]
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
