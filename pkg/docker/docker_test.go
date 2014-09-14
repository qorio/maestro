package docker

import (
	"encoding/json"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestDocker(t *testing.T) { TestingT(t) }

type DockerTests struct{}

var _ = Suite(&DockerTests{})

const DOCKER_EMAIL = "davidc616@gmail.com"
const DOCKER_AUTH = "bGFiNjE2OmxhYjYxNg=="

func (suite *DockerTests) TestGenerateDockerCfg(c *C) {
	file := filepath.Join(c.MkDir(), ".dockercfg")

	config := &Config{Email: DOCKER_EMAIL, Auth: DOCKER_AUTH}
	err := config.GenerateDockerCfg(file)
	c.Assert(err, Equals, nil)

	fi, err := os.Stat(file)
	c.Assert(err, Equals, nil)
	c.Assert(fi.IsDir(), Equals, false)

	f, err := os.Open(file)
	c.Assert(err, Equals, nil)

	buff, err := ioutil.ReadAll(f)
	c.Assert(err, Equals, nil)

	cfg := make(map[string]interface{})
	err = json.Unmarshal(buff, &cfg)
	c.Assert(err, Equals, nil)

	m := cfg["https://index.docker.io/v1/"].(map[string]interface{})
	c.Assert(m["auth"], Equals, DOCKER_AUTH)
	c.Assert(m["email"], Equals, DOCKER_EMAIL)
}
