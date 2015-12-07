package zk

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/qorio/maestro/pkg/registry"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func TestFuse(t *testing.T) { TestingT(t) }

type TestSuiteFuse struct {
	zc ZK
}

var _ = Suite(&TestSuiteFuse{})

func (suite *TestSuiteFuse) SetUpSuite(c *C) {
	zc, err := Connect(ZkHosts(), 1*time.Second)
	c.Assert(err, Equals, nil)
	suite.zc = zc
	// Create a filesystem
	CreateOrSet(suite.zc, "/unit-test/fuse/dir1/file1", "dir1-file1")
	CreateOrSet(suite.zc, "/unit-test/fuse/dir1/file2", "dir1-file2")
	CreateOrSet(suite.zc, "/unit-test/fuse/dir2/file1", "dir2-file1")
	CreateOrSet(suite.zc, "/unit-test/fuse/dir2/file2", "dir2-file2")
	CreateOrSet(suite.zc, "/unit-test/fuse/dir2/dir-a/file1", "dir2-dir-a-file1")
	CreateOrSet(suite.zc, "/unit-test/fuse/dir2/dir-a/file2", "dir2-dir-a-file2")
	CreateOrSet(suite.zc, "/unit-test/fuse/dir2/dir-a/file3", "dir2-dir-a-file3")
}

func (suite *TestSuiteFuse) TearDownSuite(c *C) {
	suite.zc.Close()
}

func (suite *TestSuiteFuse) _TestMount(c *C) {
	dir := c.MkDir()
	c.Log("Dir=", dir)
	fc, err := fuse.Mount(dir)
	c.Assert(err, Equals, nil)

	filesys := &FS{fsNode: fs_node(suite.zc, nil, registry.Path("/unit-test/fuse"), nil)}

	c.Log("Start serving")
	err = fs.Serve(fc, filesys)
	c.Assert(err, Equals, nil)

	c.Log("Error=", fc.MountError)
}
