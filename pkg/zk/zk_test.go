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

func (suite *ZkTests) TestFullPathObjects(c *C) {
	z, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	defer z.Close()

	top, err := z.Get("/dir1")
	if err == ErrNotExist {
		top, err = z.Create("/dir1", nil)
		c.Assert(err, Equals, nil)
	}
	c.Assert(top, Not(Equals), (*Node)(nil))
	all_children, err := top.ChildrenRecursive()
	c.Assert(err, Equals, nil)
	for _, n := range all_children {
		c.Log("Deleting", n.Path)
		err := n.Delete()
		c.Assert(err, Equals, nil)
	}

	path := "/dir1/dir2/dir3"
	value := []byte(path)
	v, err := z.Create(path, value)
	c.Assert(err, Equals, nil)
	c.Assert(v, Not(Equals), nil)

	for i := 0; i < 5; i++ {
		k := fmt.Sprintf("/dir1/dir2/dir3/dir4/%04d", i)
		v := fmt.Sprintf("%s", i)
		_, err := z.Create(k, []byte(v))
		c.Assert(err, Equals, nil)
	}
	// Get the full list of children
	paths, err := v.ListAllRecursive()
	c.Assert(err, Equals, nil)
	for _, p := range paths {
		_, err := z.Get(p)
		c.Assert(err, Equals, nil)
		c.Log("> ", p)
	}
}

func (suite *ZkTests) TestAppEnvironments(c *C) {
	z, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	defer z.Close()

	// Common use case of storing properties under different environments
	expects := map[string]string{
		"/environments/integration/database/driver":     "postgres",
		"/environments/integration/database/dbname":     "pg_db",
		"/environments/integration/database/user":       "pg_user",
		"/environments/integration/database/password":   "password",
		"/environments/integration/database/port":       "5432",
		"/environments/integration/env/S3_API_KEY":      "s3-api-key",
		"/environments/integration/env/S3_API_SECRET":   "s3-api-secret",
		"/environments/integration/env/SERVICE_API_KEY": "service-api-key",
	}

	for k, v := range expects {
		_, err = z.Create(k, []byte(v))
		c.Log(k, "err", err)
		//c.Assert(err, Equals, nil)
	}

	integration, err := z.Get("/environments/integration")
	c.Assert(err, Equals, nil)

	all, err := integration.FilterChildrenRecursive(func(z *Node) bool {
		return !z.IsLeaf() // filter out parent nodes
	})
	c.Assert(err, Equals, nil)

	for _, n := range all {
		c.Log(n.GetBasename(), "=", string(n.Value))
	}
	c.Assert(len(all), Equals, len(expects)) // exactly as the map since we filter out the parent node /database

	for _, n := range all {
		err = n.Delete()
		c.Assert(err, Equals, nil)
	}
}

func (suite *ZkTests) TestEphemeral(c *C) {
	z1, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	p := "/e1/e2"
	top1, err := z1.Get(p)
	if err == ErrNotExist {
		top1, err = z1.Create(p, nil)
		c.Assert(err, Equals, nil)
	}
	err = top1.Get()
	c.Assert(err, Equals, nil)
	c.Log("top1", top1)

	top11, err := z1.CreateEphemeral(p+"/11", nil)
	c.Assert(err, Equals, nil)
	c.Log("top1", top11)

	z2, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)
	top2, err := z2.Get(p + "/11")
	c.Assert(err, Not(Equals), ErrNotExist)
	c.Log("z2 sees", top2)

	z1.Close() // the ephemeral node /11 should go away

	err = top2.Get()
	c.Log("top2", top2)
	c.Assert(err, Equals, ErrNotExist)

	// what about the static one
	top22, err := z2.Get(p)
	c.Assert(err, Not(Equals), ErrNotExist)
	c.Log("z2 sees", top22)

	z2.Close()
}

func (suite *ZkTests) TestWatcher(c *C) {
	z1, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	p := "/e1/e2"
	top1, err := z1.Get(p)
	if err == ErrNotExist {
		top1, err = z1.Create(p, nil)
		c.Assert(err, Equals, nil)
	}
	err = top1.Get()
	c.Assert(err, Equals, nil)
	c.Log("top1", top1)

	top11, err := z1.CreateEphemeral(p+"/11", nil)
	c.Assert(err, Equals, nil)
	c.Log("top1", top11)

	// Watched by another client
	z2, err := Connect([]string{"localhost:2181"}, time.Second)
	c.Assert(err, Equals, nil)

	top22, err := z2.Get(p + "/11")
	c.Assert(err, Not(Equals), ErrNotExist)
	c.Log("z2 sees", top22)

	stop22, err := top22.Watch(func(e Event) {
		if e.State != zk.StateDisconnected {
			c.Log("Got event :::::", e)
		}
	})
	c.Assert(err, Equals, nil)

	// Now do a few things
	top22.Set([]byte("New value"))

	// Now watch something else
	new_path := "/new/path/to/be/created"
	stop23, err := z2.Watch(new_path, func(e Event) {
		if e.State != zk.StateDisconnected {
			c.Log("Got event -----", e)
		}
	})
	c.Assert(err, Equals, nil)

	// Create a new node
	_, err = z1.CreateEphemeral(new_path, nil)
	c.Assert(err, Equals, nil)

	c.Log("closing z1")
	z1.Close() // the ephemeral node /11 should go away

	time.Sleep(1 * time.Second)
	c.Log("sending stop")
	stop22 <- true
	stop23 <- true
	c.Log("stop sent")
}
