package magician

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	for i := range []int{0, 1, 2} {
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

func (suite *MutateTestSuite) Test_0_ImageNoExistingCredentials() {
	img := empty.Image
	ref := *suite.TestReferences[0]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test0 setup")

	refStr := ref.String()
	err = Mutate(refStr)
	suite.Nil(err, "test0 Mutate fails")

	altTag := fmt.Sprintf("%s.magic", ref.String())
	opts := []MutateOption{MutateOptWithTag(altTag)}
	err = Mutate(refStr, opts...)
	suite.Nil(err, "test0 Mutate fails with alt tag")
}

// TODO this test
func (suite *MutateTestSuite) Test_1_ImageExistingCredentialsHomeJSON() {
	img := empty.Image
	ref := *suite.TestReferences[1]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test0 setup")
}

// TODO this test
func (suite *MutateTestSuite) Test_2_ImageExistingCredentialsEnvVar() {
	img := empty.Image
	ref := *suite.TestReferences[2]
	err := remote.Write(ref, img, suite.RemoteOpts...)
	suite.Nil(err, "remote write for test0 setup")
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
