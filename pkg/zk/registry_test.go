package zk

import (
	"fmt"
	r "github.com/qorio/maestro/pkg/registry"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) { TestingT(t) }

type RegistryTests struct {
	zk  ZK
	zk2 ZK
}

var _ = Suite(&RegistryTests{})

func (suite *RegistryTests) SetUpSuite(c *C) {
	z, err := Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)
	suite.zk = z

	z2, err := Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)
	suite.zk2 = z2
}

func test_ns(k string) r.Path {
	var now = time.Now().Unix()
	return r.Path(fmt.Sprintf("/unit-test/%d%s", now, k))
}

func (suite *RegistryTests) TearDownSuite(c *C) {
	suite.zk.Delete("/unit-test") // TODO - this fails before there are children under this node
	suite.zk.Close()
	suite.zk2.Close()
}

func (suite *RegistryTests) TestInvalidState(c *C) {

	key := test_ns("/test/delete")
	// Add Delete watch to non-existent node should give errors
	zdelete := NewDelete(r.Delete(key), suite.zk)
	c.Log("Delete:", zdelete)
	err := zdelete.Apply(func(k r.Key, before, after *Node) bool {
		return true // this doesn't make sense
	})
	c.Assert(err, Equals, ErrInvalidState)

	key = test_ns("/test/exist")
	// Add create watch to an existing node should give errors
	err = CreateOrSet(suite.zk, key, "hello")
	c.Assert(err, Equals, nil)

	zcreate := NewCreate(r.Create(key), suite.zk)
	c.Log("Create:", zcreate)
	err = zcreate.Apply(func(k r.Key, before, after *Node) bool {
		return true // this doesn't make sense
	})
	c.Assert(err, Equals, ErrInvalidState)
}

func (suite *RegistryTests) TestValidStates(c *C) {

	key := test_ns("/test/delete")
	// Add create watch to non existing node is ok
	err := DeleteObject(suite.zk, key)
	c.Assert(err, Equals, nil)

	zcreate := NewCreate(r.Create(key), suite.zk)
	c.Log("Create:", zcreate)
	err = zcreate.Apply(func(k r.Key, before, after *Node) bool {
		return true // this is ok
	})
	c.Assert(err, Equals, nil)

	key = test_ns("/test/create")
	// Add delete watch to an existing node is ok
	err = CreateOrSet(suite.zk, key, "hello")
	c.Assert(err, Equals, nil)

	zdelete := NewDelete(r.Delete(key), suite.zk)
	c.Log("Delete:", zdelete)
	err = zdelete.Apply(func(k r.Key, before, after *Node) bool {
		return true // this is ok
	})
	c.Assert(err, Equals, nil)
}

func (suite *RegistryTests) TestCreate(c *C) {
	called := make(chan bool)

	key := test_ns("/test/testCreate")
	zcreate := NewCreate(r.Create(key), suite.zk)
	c.Log("Create:", zcreate)
	err := zcreate.Apply(func(k r.Key, before, after *Node) bool {
		called <- true

		c.Assert(k.Path(), Equals, key.Path())
		c.Assert(before, Equals, (*Node)(nil))
		c.Assert(after.GetValueString(), Equals, "hello")

		return true
	})
	c.Assert(err, Equals, nil)

	// create the node
	CreateOrSet(suite.zk, key, "hello")
	c.Assert(<-called, Equals, true)

}

func (suite *RegistryTests) TestDelete(c *C) {

	key := test_ns("/test/testDelete")
	CreateOrSet(suite.zk, key, "hello")

	called := make(chan bool)

	zdelete := NewDelete(r.Delete(key), suite.zk)
	c.Log("Delete:", zdelete)
	err := zdelete.Apply(func(k r.Key, before, after *Node) bool {
		called <- true

		c.Assert(k.Path(), Equals, key.Path())
		c.Assert(before.GetValueString(), Equals, "hello")
		c.Assert(after, Equals, (*Node)(nil))

		return true
	})

	err = DeleteObject(suite.zk, key)
	c.Assert(err, Equals, nil)
	c.Assert(<-called, Equals, true)

}

func (suite *RegistryTests) TestChange(c *C) {

	called := make(chan bool)

	/// The change semantics also applies to create (nil to !nil)
	key := test_ns("/test/testChange")
	zchange := NewChange(r.Change(key), suite.zk)
	c.Log("Change:", zchange)
	err := zchange.Apply(func(k r.Key, before, after *Node) bool {
		called <- true

		c.Assert(k.Path(), Equals, key.Path())
		c.Assert(after.GetValueString(), Equals, "hello")
		c.Assert(before, Equals, (*Node)(nil))

		return true
	})

	err = CreateOrSet(suite.zk, key, "hello")
	c.Assert(err, Equals, nil)
	c.Assert(<-called, Equals, true)

}

func (suite *RegistryTests) TestChange2(c *C) {

	called := make(chan bool)
	key := test_ns("/test/testChange2")
	CreateOrSet(suite.zk, key, "hello")

	zchange := NewChange(r.Change(key), suite.zk)
	c.Log("Change:", zchange)
	err := zchange.Apply(func(k r.Key, before, after *Node) bool {
		called <- true
		c.Assert(k.Path(), Equals, key.Path())
		c.Assert(after.GetValueString(), Equals, "there")
		c.Assert(before.GetValueString(), Equals, "hello")

		return true
	})

	err = CreateOrSet(suite.zk, key, "there")
	c.Assert(err, Equals, nil)
	c.Assert(<-called, Equals, true)
}

func (suite *RegistryTests) TestMembers(c *C) {

	called := make(chan bool)
	key := test_ns("/test/testMembers1")
	CreateOrSet(suite.zk, key, "hello")

	zmembers := NewMembers(r.Members{Top: key}, suite.zk)
	c.Log("Members:", zmembers)
	err := zmembers.Apply(func(k r.Key, before, after *Node) bool {
		called <- true

		c.Assert(k.Path(), Equals, key.Path())
		c.Assert(before.Stats.NumChildren, Equals, int32(0))
		c.Assert(after.Stats.NumChildren, Equals, int32(1))

		return true
	})

	child := r.Path(key.Path() + "/child1")
	err = CreateOrSet(suite.zk2, child, "hello")
	c.Assert(err, Equals, nil)
	c.Log("added member", child)

	c.Assert(<-called, Equals, true)
}

func (suite *RegistryTests) TestMembers2(c *C) {

	// Node not existing. Wait for creation and then resubscribe to children
	called := make(chan bool)
	key := test_ns("/test/testMembers2")

	zmembers := NewMembers(r.Members{Top: key}, suite.zk)
	c.Log("Members:", zmembers)
	err := zmembers.Apply(func(k r.Key, before, after *Node) bool {
		called <- true

		c.Assert(k.Path(), Equals, key.Path())
		c.Assert(before, Equals, (*Node)(nil))
		c.Assert(after.Stats.NumChildren, Equals, int32(1))

		return true
	})

	child := r.Path(key.Path() + "/child1")
	err = CreateOrSet(suite.zk2, child, "hello")
	c.Assert(err, Equals, nil)
	c.Log("added member", child)

	c.Assert(<-called, Equals, true)
}
