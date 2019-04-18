package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/registry"
	_ "github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
)

var (
	testConfig   = "test.config"
	testHtpasswd = "test.htpasswd"
	testUsername = "alice"
	testPassword = "wonderland"
)

type DockerClientTestSuite struct {
	suite.Suite
	DockerRegistryHost string
	Client             *Client
	TempTestDir        string
}

func newContext() context.Context {
	return context.Background()
}

func (suite *DockerClientTestSuite) SetupSuite() {
	// Temporarily move docker conf dir (if exists)
	dockerDir := config.Dir()
	if _, err := os.Stat(dockerDir); !os.IsNotExist(err) {
		err = os.Rename(dockerDir, dockerDir+"_oras_backup")
		suite.Nil(err, "no error moving docker dir")

		// Make empty docker dir
		err = os.MkdirAll(dockerDir, 0700)
		suite.Nil(err, "no error making empty docker dir")
	}

	tempDir, err := ioutil.TempDir("", "oras_auth_docker_test")
	suite.Nil(err, "no error creating temp directory for test")
	suite.TempTestDir = tempDir

	// Create client
	client, err := NewClient(filepath.Join(suite.TempTestDir, testConfig))
	suite.Nil(err, "no error creating client")
	var ok bool
	suite.Client, ok = client.(*Client)
	suite.True(ok, "NewClient returns a *docker.Client inside")

	// Create htpasswd file with bcrypt
	secret, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	suite.Nil(err, "no error generating bcrypt password for test htpasswd file")
	authRecord := fmt.Sprintf("%s:%s\n", testUsername, string(secret))
	htpasswdPath := filepath.Join(suite.TempTestDir, testHtpasswd)
	err = ioutil.WriteFile(htpasswdPath, []byte(authRecord), 0644)
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
	suite.Nil(err, "no error finding free port for test registry")

	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}

func (suite *DockerClientTestSuite) TearDownSuite() {

	// Move docker conf dir back if necessary
	dockerDir := config.Dir()
	if _, err := os.Stat(dockerDir + "_oras_backup"); !os.IsNotExist(err) {
		err = os.RemoveAll(dockerDir)
		suite.Nil(err, "no error removing test docker dir")
		err = os.Rename(dockerDir+"_oras_backup", dockerDir)
		suite.Nil(err, "no error restoring docker dir")
	}

	os.RemoveAll(suite.TempTestDir)
}


func (suite *DockerClientTestSuite) Test_0_Resolver() {
	_, err := suite.Client.Resolver(newContext())
	suite.Nil(err, "no error retrieving resolver")
}

func (suite *DockerClientTestSuite) Test_1_Login() {
	var err error

	err = suite.Client.Login(newContext(), suite.DockerRegistryHost, "oscar", "opponent")
	suite.NotNil(err, "error logging into registry with invalid credentials")

	err = suite.Client.Login(newContext(), suite.DockerRegistryHost, "", testPassword)
	suite.NotNil(err, "error logging into registry with no username")

	err = suite.Client.Login(newContext(), suite.DockerRegistryHost, testUsername, testPassword)
	suite.Nil(err, "no error logging into registry with valid credentials")
}

func (suite *DockerClientTestSuite) Test_2_Credential() {
	username, password, err := suite.Client.Credential(suite.DockerRegistryHost)
	suite.Nil(err, "no error getting credentials")
	suite.Equal(testUsername, username, "username matches")
	suite.Equal(testPassword, password, "password matches")

	username, password, err = suite.Client.Credential("mybadhost:54321")
	suite.Nil(err, "no error getting credentials")
	suite.Equal("", username, "username empty with unauthed host")
	suite.Equal("", password, "password empty with unauthed host")

	username, password, err = suite.Client.Credential("index.docker.io")
	suite.Nil(err, "no error getting credentials")
}

func (suite *DockerClientTestSuite) Test_3_Logout() {
	var err error

	err = suite.Client.Logout(newContext(), "non-existing-host:42")
	suite.NotNil(err, "error logging out of registry that has no entry")

	err = suite.Client.Logout(newContext(), suite.DockerRegistryHost)
	suite.Nil(err, "no error logging out of registry")
}

func (suite *DockerClientTestSuite) Test_4_NewClient() {
	_, err := NewClient("/")
	suite.NotNil(err, "error creating client with bad config path (root)")

	_, err = NewClient()
	suite.Nil(err, "no error with no config paths")

	badConfigPath := filepath.Join(config.Dir(), "fake.json")
	_, err = os.Create(badConfigPath)

	err = os.Chmod(config.Dir(), 0400)
	suite.Nil(err, "no error chmod test config file")
	_, err = NewClient(badConfigPath)
	suite.NotNil(err, "error with bad config paths, 0400 perms")

	err = os.Chmod(config.Dir(), 0400)
	suite.Nil(err, "no error chmod test docker dir")
	_, err = NewClient()
	suite.NotNil(err, "error when docker dir has bad perms")

	err = os.Chmod(config.Dir(), 0700)
	suite.Nil(err, "no error chmod test docker dir (back)")
}

func TestDockerClientTestSuite(t *testing.T) {
	suite.Run(t, new(DockerClientTestSuite))
}
