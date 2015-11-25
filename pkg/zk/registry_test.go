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
	z, err := Connect(ZkHosts(), 5*time.Second)
	c.Assert(err, Equals, nil)
	suite.zk = z

	z2, err := Connect(ZkHosts(), 5*time.Second)
	c.Assert(err, Equals, nil)
	suite.zk2 = z2
}

func test_ns(k string) r.Path {
	var now = time.Now().Unix()
	return r.Path(fmt.Sprintf("/unit-test/%d%s", now, k))
}

func (suite *RegistryTests) TearDownSuite(c *C) {
	suite.zk.Delete("/unit-test") // TODO - this fails before there are children under this node
	c.Log("Closing")
	suite.zk.Close()
	c.Log("Closing2")
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

func (suite *RegistryTests) TestChangeSimple(c *C) {

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

func (suite *RegistryTests) TestChangeAlsoImpliesCreate(c *C) {

	called := make(chan bool)

	/// The change semantics also applies to create (nil to !nil)
	key := test_ns("/test/testChange")

	// This node should not exist
	DeleteObject(suite.zk, key)

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

func (suite *RegistryTests) TestMembersAlsoImpliesCreate(c *C) {

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

func (suite *RegistryTests) TestTimeout(c *C) {

	timeout := r.Timeout(1 * time.Second)
	create := r.Create(test_ns("/conditions1/test/create"))
	cond := r.Conditions{
		Timeout: &timeout,
		Create:  &create,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	time.Sleep(10)

	err := <-received
	c.Assert(err, Equals, ErrTimeout)

}

func (suite *RegistryTests) TestConditions1(c *C) {

	timeout := r.Timeout(10 * time.Second)
	create := r.Create(test_ns("/conditions1/test/create"))
	cond := r.Conditions{
		Timeout: &timeout,
		Create:  &create,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	// Now create the node
	CreateOrSet(suite.zk, test_ns("/conditions1/test/create"), "foo")
	err := <-received

	c.Assert(err, Equals, nil)
}

func (suite *RegistryTests) TestConditions2(c *C) {

	timeout := r.Timeout(2 * time.Second)
	create := r.Create(test_ns("/conditions2/test/create"))
	delete := r.Delete(test_ns("/conditions2/test/create"))
	cond := r.Conditions{
		All:     true,
		Timeout: &timeout,
		Create:  &create,
		Delete:  &delete,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	// Now create the node
	CreateOrSet(suite.zk, test_ns("/conditions2/test/create"), "foo")

	// Now let it timeout
	time.Sleep(5)
	err := <-received
	c.Assert(err, Equals, ErrTimeout)
}

func (suite *RegistryTests) TestConditions3(c *C) {

	p1, p2 := test_ns("/conditions3/test/1"), test_ns("/conditions3/test/2")
	CreateOrSet(suite.zk, p1, "bar")

	timeout := r.Timeout(10 * time.Second)
	delete := r.Delete(p1)
	create := r.Create(p2)

	cond := r.Conditions{
		All:     true,
		Timeout: &timeout,
		Create:  &create,
		Delete:  &delete,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	// Now create the node
	CreateOrSet(suite.zk, p2, "foo")

	// Now let it timeout
	time.Sleep(2)

	DeleteObject(suite.zk, p1)

	err := <-received

	c.Log("Pending=", conditions.Pending())
	c.Assert(err, Equals, nil)

}

/// Use case for counting members in a load balancer group
func (suite *RegistryTests) TestConditionsMembersMin(c *C) {

	p := test_ns("/conditions4/test/group")
	CreateOrSet(suite.zk, p, "bar")

	min := int32(2)
	timeout := r.Timeout(10 * time.Second)
	members := r.Members{
		Top: p,
		Min: &min,
	}

	cond := r.Conditions{
		Timeout: &timeout,
		Members: &members,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	// Now create the node
	CreateOrSet(suite.zk, p.Member("a"), "foo")
	CreateOrSet(suite.zk, p.Member("b"), "foo")
	CreateOrSet(suite.zk, p.Member("c"), "foo")

	// Now let it timeout
	time.Sleep(2)

	err := <-received

	c.Assert(err, Equals, nil)
}

/// Use case for looking for a delta change in members
func (suite *RegistryTests) TestConditionsDeltaMembers(c *C) {

	p := test_ns("/deltas/test/group")
	CreateOrSet(suite.zk, p, "bar")
	CreateOrSet(suite.zk, p.Member("a"), "foo")
	CreateOrSet(suite.zk, p.Member("b"), "foo")

	delta := int32(1)
	timeout := r.Timeout(10 * time.Second)
	members := r.Members{
		Top:   p,
		Delta: &delta,
	}

	cond := r.Conditions{
		Timeout: &timeout,
		Members: &members,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	go func() {
		time.Sleep(1)

		// Set existing
		CreateOrSet(suite.zk2, p.Member("a"), "foo")

		// Delete
		DeleteObject(suite.zk2, p.Member("b"))

		// Now create the node
		CreateOrSet(suite.zk2, p.Member("c"), "foo")
	}()

	err := <-received

	c.Assert(err, Equals, nil)
}

/// Use case for looking for a delta change in members
func (suite *RegistryTests) DISABLE_TestConditionsDeltaMembersLessOneMultipleOps(c *C) {

	p := test_ns("/deltas/test/group")
	CreateOrSet(suite.zk, p, "bar")
	CreateOrSet(suite.zk, p.Member("a"), "foo")
	CreateOrSet(suite.zk, p.Member("b"), "foo")
	CreateOrSet(suite.zk, p.Member("c"), "foo")

	delta := int32(-1)
	timeout := r.Timeout(10 * time.Second)
	members := r.Members{
		Top:   p,
		Delta: &delta,
	}

	cond := r.Conditions{
		Timeout: &timeout,
		Members: &members,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	go func() {

		time.Sleep(1)

		c.Log("Existing")
		// Set existing
		CreateOrSet(suite.zk2, p.Member("c"), "foo")

		time.Sleep(1)

		// TODO  - this will break the test
		c.Log("Add new")
		// Add new node
		CreateOrSet(suite.zk2, p.Member("z"), "foo")

		time.Sleep(1)

		c.Log("Remove")
		// Now remove the node
		DeleteObject(suite.zk2, p.Member("b"))

	}()

	err := <-received
	c.Assert(err, Equals, nil)
	c.Log("Received 1 delta")
}

/// Use case for looking for a delta change in members
func (suite *RegistryTests) TestConditionsDeltaMembersLessOneSimple(c *C) {

	p := test_ns("/deltas/test/group")
	CreateOrSet(suite.zk, p, "bar")
	CreateOrSet(suite.zk, p.Member("a"), "foo")
	CreateOrSet(suite.zk, p.Member("b"), "foo")
	CreateOrSet(suite.zk, p.Member("c"), "foo")

	delta := int32(-1)
	timeout := r.Timeout(10 * time.Second)
	members := r.Members{
		Top:   p,
		Delta: &delta,
	}

	cond := r.Conditions{
		Timeout: &timeout,
		Members: &members,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
	}()

	go func() {

		time.Sleep(1)

		c.Log("Remove")
		// Now remove the node
		DeleteObject(suite.zk2, p.Member("b"))
	}()

	err := <-received
	c.Assert(err, Equals, nil)
	c.Log("Received 1 delta")
}

func (suite *RegistryTests) TestConditionsMembersEquals(c *C) {

	p := test_ns("/conditions4/test/group")
	CreateOrSet(suite.zk, p, "bar")

	eq := int32(5)
	timeout := r.Timeout(10 * time.Second)
	members := r.Members{
		Top:    p,
		Equals: &eq,
	}

	cond := r.Conditions{
		Timeout: &timeout,
		Members: &members,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
		c.Log("Got done!")
	}()

	ticker := time.NewTicker(time.Millisecond * 500)
	i := int32(0)
	for {
		done := false
		select {
		case <-ticker.C:
			// Now create the node
			CreateOrSet(suite.zk, p.Member(fmt.Sprintf("%d", i)), "foo")
			i += 1
		case <-received:
			c.Log("Received done!")
			done = true
		}
		if done {
			break
		}
	}

	c.Assert(i, DeepEquals, eq)
}

func (suite *RegistryTests) TestConditionsMemberOutsideRange(c *C) {

	p := test_ns("/conditions5/test/group")

	// Initially we want to be in range.
	// We want to trigger when it's outside this.
	CreateOrSet(suite.zk, p, "foo")
	for i := 0; i < 10; i++ {
		CreateOrSet(suite.zk, p.Member(fmt.Sprintf("%d", i)), "foo")
	}

	min := int32(3)
	max := int32(15)

	timeout := r.Timeout(10 * time.Second)
	members := r.Members{
		Top:          p,
		Min:          &min,
		Max:          &max,
		OutsideRange: true,
	}

	cond := r.Conditions{
		Timeout: &timeout,
		Members: &members,
	}

	received := make(chan error)

	conditions := NewConditions(cond, suite.zk)
	go func() {
		received <- conditions.Wait()
		c.Log("Got done!")
	}()

	ticker := time.NewTicker(time.Millisecond * 500)

	// Let grow ==> move out of range by growing...
	n := int32(0)
	for {
		done := false
		select {
		case <-ticker.C:
			// Now create the node
			CreateOrSet(suite.zk, p.Member(fmt.Sprintf("%d", n+10)), "foo")
			n++
		case <-received:
			c.Log("Received done!")
			done = true
		}
		if done {
			break
		}
	}

	c.Assert(n, Equals, int32(5))
}
