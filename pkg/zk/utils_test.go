package zk

import (
	"github.com/qorio/maestro/pkg/registry"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func TestUtils(t *testing.T) { TestingT(t) }

type TestSuiteUtils struct {
	zc ZK
}

var _ = Suite(&TestSuiteUtils{})

func (suite *TestSuiteUtils) SetUpSuite(c *C) {
	zc, err := Connect(ZkHosts(), 1*time.Second)
	c.Assert(err, Equals, nil)
	suite.zc = zc
}

func (suite *TestSuiteUtils) TearDownSuite(c *C) {
	suite.zc.Close()
}

func (suite *TestSuiteUtils) TestFollow(c *C) {
	CreateOrSet(suite.zc, "/unit-test/follow/1", "found!")
	CreateOrSet(suite.zc, "/unit-test/follow/2", "env:///unit-test/follow/1")
	CreateOrSet(suite.zc, "/unit-test/follow/3", "env:///unit-test/follow/2")
	CreateOrSet(suite.zc, "/unit-test/follow/4", "env:///unit-test/follow/3")
	CreateOrSet(suite.zc, "/unit-test/follow/5", "env:///unit-test/follow/4")

	n, err := Follow(suite.zc, registry.Path("/unit-test/follow/5"))
	c.Assert(err, Equals, nil)
	c.Assert(n.GetValueString(), Equals, "found!")
}

func (suite *TestSuiteUtils) TestResolve(c *C) {
	CreateOrSet(suite.zc, "/unit-test/resolve/1", "found!")
	CreateOrSet(suite.zc, "/unit-test/resolve/2", "env:///unit-test/resolve/1")
	CreateOrSet(suite.zc, "/unit-test/resolve/3", "env:///unit-test/resolve/2")
	CreateOrSet(suite.zc, "/unit-test/resolve/4", "env:///unit-test/resolve/3")
	CreateOrSet(suite.zc, "/unit-test/resolve/5", "env:///unit-test/resolve/4")

	p, v, err := Resolve(suite.zc, registry.Path("/unit-test/resolve/5"), "env:///unit-test/resolve/4")
	c.Assert(err, Equals, nil)
	c.Assert(p.Path(), Equals, "/unit-test/resolve/5")
	c.Assert(v, Equals, "found!")
}
