package zk

import (
	"fmt"
	"github.com/samuel/go-zookeeper/zk"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func TestZk(t *testing.T) { TestingT(t) }

type ZkTests struct{}

var _ = Suite(&ZkTests{})

func (suite *ZkTests) TestConnect(c *C) {
	z, err := Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)
	c.Log("Got client", z)
	c.Assert(z.conn, Not(Equals), nil)
	z.Close()
	c.Assert(z.conn, Equals, (*zk.Conn)(nil))

	// Reconnect
	err = z.Reconnect()
	c.Assert(err, Equals, nil)
	c.Assert(z.conn, Not(Equals), nil)
}

func (suite *ZkTests) TestBasicOperations(c *C) {
	z, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	defer z.Close()

	path := "/test"
	value := []byte("/test")
	v, err := z.Get(path)
	c.Assert(err, Not(Equals), ErrNotConnected)

	if err == ErrNotExist {
		v, err = z.Create(path, value)
		c.Assert(err, Equals, nil)
		c.Assert(v, Not(Equals), nil)
	}

	// Now create a bunch of children
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("/test/%d", i)
		data := fmt.Sprintf("value-test-%04d", i)

		x, err := z.Get(k)
		if err == ErrNotExist {
			x, err := z.Create(k, []byte(data))
			c.Assert(err, Equals, nil)
			err = x.Get()
			c.Assert(err, Equals, nil)
			c.Assert(string(x.Value), Equals, data)
		} else {
			// update
			err = x.Set([]byte(data))
			c.Assert(err, Equals, nil)
			c.Assert(string(x.Value), Equals, data)
		}
	}

	// Get children
	children, err := v.ChildrenRecursive()
	c.Assert(err, Equals, nil)
	for _, n := range children {
		c.Assert(n.CountChildren(), Equals, int32(0)) // expects leaf nodes
	}

	// Get the full list of children
	paths, err := v.ListAllRecursive()
	c.Assert(err, Equals, nil)
	for _, p := range paths {
		_, err := z.Get(p)
		c.Assert(err, Equals, nil)
	}

	all_children, err := v.ChildrenRecursive()
	c.Assert(err, Equals, nil)
	for _, n := range all_children {
		err := n.Delete()
		c.Assert(err, Equals, nil)
	}
}

func (suite *ZkTests) __TestFullPathObjects(c *C) {
	z, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	defer z.Close()

	path := "/dir1/dir2/dir3"
	value := []byte(path)

	v, err := z.Get(path)
	c.Assert(err, Not(Equals), ErrNotConnected)

	if err == ErrNotExist {
		v, err = z.Create(path, value)
		c.Assert(err, Equals, nil)
		c.Assert(v, Not(Equals), nil)
	}

	// Now create a bunch of children
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf(path+"/test/%d", i)
		data := fmt.Sprintf("value-test-%04d", i)

		x, err := z.Get(k)
		if err == ErrNotExist {
			x, err := z.Create(k, []byte(data))
			err = x.Get()
			c.Assert(err, Equals, nil)
			c.Assert(string(x.Value), Equals, data)
		} else {
			// update
			err = x.Set([]byte(data))
			c.Assert(err, Equals, nil)
			c.Assert(string(x.Value), Equals, data)
		}
	}

	// Get children
	children, err := v.ChildrenRecursive()
	c.Assert(err, Equals, nil)
	for _, n := range children {
		c.Assert(n.CountChildren(), Equals, int32(0)) // expects leaf nodes
	}

	// Get the full list of children
	paths, err := v.ListAllRecursive()
	c.Assert(err, Equals, nil)
	for _, p := range paths {
		_, err := z.Get(p)
		c.Assert(err, Equals, nil)
		c.Log("> ", p)
	}

	all_children, err := v.ChildrenRecursive()
	c.Assert(err, Equals, nil)
	for _, n := range all_children {
		err := n.Delete()
		c.Assert(err, Equals, nil)
	}
}
