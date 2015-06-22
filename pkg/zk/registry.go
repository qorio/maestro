package zk

import (
	"errors"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/samuel/go-zookeeper/zk"
	"time"
)

var (
	ErrNotInitialized = errors.New("not-initialized")
	ErrNotWatching    = errors.New("not-watching")
	ErrInvalidState   = errors.New("invalid-state")
	ErrTimeout        = errors.New("timeout")
)

type watch interface {
	String() string
	SetTimeout(time.Duration) error
	Apply(func(k registry.Key, before, after *Node) bool) error
	SetGroupChan(chan<- watch)
	Wait() error
}

// Implements some utilities for the registry types
type base struct {
	zk      ZK
	stop    chan<- bool
	timeout time.Duration
	timer   *time.Timer
	before  *Node
	after   *Node
	done    chan error
	group   chan<- watch // for sending to the group
	error   error
}

type Conditions struct {
	registry.Conditions

	watches map[watch]bool
	group   chan watch
	timer   *time.Timer
}

func (this *Conditions) Pending() []watch {
	pending := []watch{}
	for k, _ := range this.watches {
		pending = append(pending, k)
	}
	return pending
}

type Delete struct {
	base
	registry.Delete
}

type Create struct {
	registry.Create
	base
}

type Change struct {
	base
	registry.Change
}

type Members struct {
	base
	registry.Members
}

func (this *base) Init(zkc ZK) {
	this.zk = zkc
	this.done = make(chan error)
}

func (this *base) wait() error {
	for {
		return <-this.done
	}
}

func (this *Conditions) Init(zkc ZK) *Conditions {
	if this.watches == nil {
		this.watches = map[watch]bool{}
	}
	if this.Delete != nil {
		w := NewDelete(*this.Delete, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.V(100).Infoln("Delete:", k.Path(), "Before=", before, "After=", after)
			return true
		})
		this.watches[w] = false
	}
	if this.Create != nil {
		w := NewCreate(*this.Create, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.V(100).Infoln("Create:", k.Path(), "Before=", before, "After=", after)
			return true
		})
		this.watches[w] = false
	}
	if this.Change != nil {
		w := NewChange(*this.Change, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.V(100).Infoln("Change:", k.Path(), "Before=", before, "After=", after)
			return true
		})
		this.watches[w] = false
	}
	if this.Members != nil {
		w := NewMembers(*this.Members, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.V(100).Infoln("Members:", k.Path(), "Before=", before, "After=", after)

			met := false
			switch {
			case this.Members.Equals != nil:
				met = after.Stats.NumChildren == *this.Members.Equals
			case this.Members.Max != nil && this.Members.Min != nil:
				if this.Members.OutsideRange {
					met = after.Stats.NumChildren < *this.Members.Min || after.Stats.NumChildren >= *this.Members.Max
				} else {
					met = after.Stats.NumChildren >= *this.Members.Min && after.Stats.NumChildren < *this.Members.Max
				}
			case this.Members.Max != nil:
				if this.Members.OutsideRange {
					met = after.Stats.NumChildren >= *this.Members.Max
				} else {
					met = after.Stats.NumChildren < *this.Members.Max
				}
			case this.Members.Min != nil:
				if this.Members.OutsideRange {
					met = after.Stats.NumChildren < *this.Members.Min
				} else {
					met = after.Stats.NumChildren >= *this.Members.Min
				}
			}
			return met
		})
		this.watches[w] = false
	}

	this.timer = time.NewTimer(1 * time.Second)
	this.timer.Stop()
	if this.Timeout != nil {
		this.timer.Reset(time.Duration(*this.Timeout))
	}

	// for group synchronization
	this.group = make(chan watch)
	for w, _ := range this.watches {
		w.SetGroupChan(this.group)
	}

	return this
}

// Simply blocks until it's either true or a timeout occurs.
// The error will indicate whether the condition is met or a timeout took place.
func (this *Conditions) Wait() error {
	for {
		select {
		case w := <-this.group:
			if _, has := this.watches[w]; has {
				delete(this.watches, w)
			} else {
				panic(ErrInvalidState)
			}

			if !this.All || len(this.watches) == 0 {
				return nil
			}

		case <-this.timer.C:
			return ErrTimeout
		}
	}
}

func (this *Create) Wait() error {
	return this.base.wait()
}
func (this *Delete) Wait() error {
	return this.base.wait()
}
func (this *Change) Wait() error {
	return this.base.wait()
}
func (this *Members) Wait() error {
	return this.base.wait()
}

func (this *Create) String() string {
	return this.Create.Path()
}
func (this *Delete) String() string {
	return this.Delete.Path()
}
func (this *Change) String() string {
	return this.Change.Path()
}
func (this *Members) String() string {
	return this.Members.Path()
}

func NewConditions(c registry.Conditions, zkc ZK) *Conditions {
	conditions := &Conditions{Conditions: c}
	conditions.Init(zkc)
	return conditions
}

func NewCreate(c registry.Create, zkc ZK) watch {
	create := &Create{Create: c}
	create.base.Init(zkc)
	return create
}

func NewDelete(d registry.Delete, zkc ZK) watch {
	delete := &Delete{Delete: d}
	delete.base.Init(zkc)
	return delete
}

func NewChange(c registry.Change, zkc ZK) watch {
	change := &Change{Change: c}
	change.base.Init(zkc)
	return change
}

func NewMembers(m registry.Members, zkc ZK) watch {
	members := &Members{Members: m}
	members.base.Init(zkc)
	return members
}

func (this *Delete) SetGroupChan(c chan<- watch) {
	this.base.group = c
}

func (this *Create) SetGroupChan(c chan<- watch) {
	this.base.group = c
}

func (this *Change) SetGroupChan(c chan<- watch) {
	this.base.group = c
}

func (this *Members) SetGroupChan(c chan<- watch) {
	this.base.group = c
}

func (this *base) SetTimeout(t time.Duration) error {
	if this.zk == nil {
		return ErrNotInitialized
	}
	this.timeout = t
	if this.timer == nil {
		this.timer = time.AfterFunc(this.timeout, func() {
			this.cancel() // when timer fires
		})
		if this.stop != nil {
			this.timer.Reset(this.timeout) // reset as we are already watching
		} else {
			this.timer.Stop() // dont start until we are watching
		}
	}
	return nil
}

func (this *base) notify(w watch) {
	if this.group != nil {
		this.group <- w
	}
}

func (this *base) cancel() error {
	if this.zk == nil {
		return ErrNotInitialized
	}
	if this.stop == nil {
		return ErrNotWatching
	}
	this.stop <- true
	return nil
}

// This does not require the node at path to exist
func (this *base) watch(path registry.Key, handler func(Event)) (node *Node, err error) {
	if this.zk == nil {
		return nil, ErrNotInitialized
	}

	// Get the node
	node, err = this.zk.Get(path.Path())
	switch {
	case err == nil:

		switch path.(type) {
		// Specical case.  Node exists but watching for create
		case registry.Create:
			this.before = nil
			return nil, ErrInvalidState

		case registry.Delete, registry.Change:
			this.stop, err = node.Watch(handler)
			if err != nil {
				return
			}
			this.before = node

		case registry.Members:
			this.stop, err = node.WatchChildren(handler)
			if err != nil {
				return
			}
			this.before = node

		}
		if this.timer != nil {
			this.timer.Reset(this.timeout)
		}

	case err == ErrNotExist:

		switch path.(type) {
		// Specical case.  Node does not exist but watching for delete
		case registry.Delete:
			return nil, ErrInvalidState

		case registry.Create, registry.Change:
			this.stop, err = this.zk.Watch(path.Path(), handler)
			if err != nil {
				return
			}

		case registry.Members:
			this.stop, err = this.zk.WatchChildren(path.Path(), handler)
			if err != nil {
				return
			}

		}
		if this.timer != nil {
			this.timer.Reset(this.timeout)
		}
	default:
		return
	}
	return
}

func (this *Delete) Apply(handler func(k registry.Key, before, after *Node) bool) error {
	f := func(e Event) {
		switch e.Type {
		case zk.EventNodeDeleted:
			if handler(*this, this.before, nil) {
				this.base.notify(this)
				this.done <- nil
			}
		}
	}
	if n, err := this.base.watch(this.Delete, f); err != nil {
		return err
	} else {
		this.before = n
		return nil
	}
}

func (this *Create) Apply(handler func(k registry.Key, before, after *Node) bool) error {
	f := func(e Event) {
		switch e.Type {
		case zk.EventNodeCreated:
			after, err := this.zk.Get(this.Create.Path())
			if err != nil {
				this.error = err
				this.done <- err
			}
			if handler(*this, nil, after) {
				this.base.notify(this)
				this.done <- nil
			}
		}
	}
	if _, err := this.base.watch(this.Create, f); err != nil {
		return err
	} else {
		return nil
	}
}

// Change notification occurs when node is CREATED or when the value is CHANGED
func (this *Change) Apply(handler func(k registry.Key, before, after *Node) bool) error {
	f := func(e Event) {
		switch e.Type {
		// Created nil->!nil or change in value
		case zk.EventNodeDataChanged, zk.EventNodeCreated:
			after, err := this.zk.Get(this.Path())
			if err != nil {
				this.error = err
				this.done <- err
			}
			if handler(*this, this.before, after) {
				this.base.notify(this)
				this.done <- nil
			}
		}
	}

	if n, err := this.base.watch(this.Change, f); err != nil {
		return err
	} else {
		this.before = n
		return nil
	}
}

func (this *Members) Apply(handler func(k registry.Key, before, after *Node) bool) error {
	f := func(e Event) {
		switch e.Type {
		case zk.EventNodeChildrenChanged:
			after, err := this.zk.Get(this.Top.Path())
			if err != nil {
				this.error = err
				this.done <- err
			}
			if handler(*this, this.before, after) {
				this.base.notify(this)
				this.done <- nil
			} else {
				// Need to register to receive the event again.
				this.Apply(handler)
			}
		}
	}

	if n, err := this.base.watch(this.Members, f); err != nil {
		return err
	} else {
		this.before = n
		return nil
	}
}
