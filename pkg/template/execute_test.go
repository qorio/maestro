package template

import (
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetch(t *testing.T) { TestingT(t) }

type TestSuiteExecute struct {
	zc zk.ZK
}

var _ = Suite(&TestSuiteExecute{})

// Database set up for circle_ci:
// psql> create role ubuntu login password 'password';
// psql> create database circle_ci with owner ubuntu encoding 'UTF8';
func (suite *TestSuiteExecute) SetUpSuite(c *C) {
	zc, err := zk.Connect(zk.ZkHosts(), 1*time.Second)
	c.Assert(err, Equals, nil)
	suite.zc = zc
}

func (suite *TestSuiteExecute) TearDownSuite(c *C) {
	suite.zc.Close()
}

func test_url(name, content string) string {
	path := filepath.Join(os.TempDir(), name)
	err := ioutil.WriteFile(path, []byte(content), 0777)
	if err != nil {
		panic(err)
	}
	return "file://" + path
}

func (suite *TestSuiteExecute) TestFetchAndExecuteTemplateMembers1(c *C) {
	test_members := `
upstream backend { {{range containers "/{{.Domain}}/{{.Service}}/v1/containers" "3000"}}
  server {{.Host}}:{{.Port}};{{end}}
}
`
	// create some nodes
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c1:3000"), "host1:8130")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c2:3000"), "host2:8131")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c3:3000"), "host3:8132")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c4:3100"), "host4:8133")

	c.Log("Data inserted into zookeeper")
	content_url := test_url("test_members.conf", test_members)
	applied, err := ExecuteUrl(suite.zc, content_url, "", map[string]string{
		"Domain": "test.com", "Service": "testapp",
	})
	c.Assert(err, Equals, nil)
	c.Log("config= ", string(applied))
}

func (suite *TestSuiteExecute) TestFetchAndExecuteTemplateMembers2(c *C) {
	test_members := `
upstream backend { {{range members "/{{.Domain}}/{{.Service}}/v1/containers"}}
  server {{hostport .GetValueString host}}:{{hostport .GetValueString port}};{{end}}
}
upstream backend { {{range members "/{{.Domain}}/{{.Service}}/v1/containers"}}
  server {{.GetValueString}};{{end}}
}
`
	// create some nodes
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c1:3000"), "host1:8130")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c2:3000"), "host2:8131")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c3:3000"), "host3:8132")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c4:3100"), "host4:8133")

	c.Log("Data inserted into zookeeper")
	content_url := test_url("test_members.conf", test_members)
	applied, err := ExecuteUrl(suite.zc, content_url, "", map[string]string{
		"Domain": "test.com", "Service": "testapp",
	})
	c.Assert(err, Equals, nil)
	c.Log("config= ", string(applied))
}

func (suite *TestSuiteExecute) TestFetchAndExecuteShell(c *C) {
	test_shell := `
{{shell "date && ls -al | wc -l"}}
`
	content_url := test_url("test_shell.conf", test_shell)
	applied, err := ExecuteUrl(suite.zc, content_url, "", map[string]string{
		"Domain": "test.com", "Service": "testapp",
	})
	c.Assert(err, Equals, nil)
	c.Log("config= ", string(applied))
}
