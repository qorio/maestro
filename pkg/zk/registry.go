package zk

import (
	"bytes"
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
)

type watch interface {
	Init(registry.Key, ZK) watch
	SetTimeout(time.Duration) error
	Apply(func(k registry.Key, before, after *Node) bool) error
	SetGroupChan(chan<- watch)
}

// Implements some utilities for the registry types
type base struct {
	zk      ZK
	stop    chan<- bool
	timeout time.Duration
	timer   *time.Timer
	before  *Node
	after   *Node
	error   chan error
	group   chan<- watch // for sending to the group
}

type Conditions struct {
	registry.Conditions

	watches map[watch]bool
	group   chan watch
}

type Delete struct {
	base
	registry.Delete
}

type Create struct {
	base
	registry.Create
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
	this.error = make(chan error)
}

func (this *Conditions) Init(zkc ZK) *Conditions {
	if this.watches == nil {
		this.watches = map[watch]bool{}
	}
	if this.Delete != nil {
		w := new(Delete).Init(*this.Delete, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.Infoln("Delete:", k.Path(), "Before=", before, "After=", after)
			return true
		})
		this.watches[w] = false
	}
	if this.Create != nil {
		w := new(Create).Init(*this.Create, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.Infoln("Create:", k.Path(), "Before=", before, "After=", after)
			return true
		})
		this.watches[w] = false
	}
	if this.Change != nil {
		w := new(Change).Init(*this.Change, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.Infoln("Change:", k.Path(), "Before=", before, "After=", after)
			return !bytes.Equal(before.Value, after.Value)
		})
		this.watches[w] = false
	}
	if this.Members != nil {
		w := new(Members).Init(*this.Members, zkc)
		w.Apply(func(k registry.Key, before, after *Node) bool {
			glog.Infoln("Members:", k.Path(), "Before=", before, "After=", after)
			switch {
			case this.Members.Equals != nil:
				return after.Stats.NumChildren == *this.Members.Equals
			case this.Members.Max != nil && this.Members.Min != nil:
				return after.Stats.NumChildren >= *this.Members.Min && after.Stats.NumChildren < *this.Members.Max
			case this.Members.Max != nil:
				return after.Stats.NumChildren < *this.Members.Max
			case this.Members.Min != nil:
				return after.Stats.NumChildren >= *this.Members.Min
			}
			return false
		})
		this.watches[w] = false
	}
	if this.Timeout != nil {
		for w, _ := range this.watches {
			w.SetTimeout(time.Duration(*this.Timeout))
		}
	}

	// for group synchronization
	this.group = make(chan watch)
	for w, _ := range this.watches {
		w.SetGroupChan(this.group)
	}

	return this
}

func (this *Conditions) Wait() {
	for {
		w := <-this.group
		if _, has := this.watches[w]; has {
			delete(this.watches, w)
		} else {
			panic(ErrInvalidState)
		}

		if !this.All || len(this.watches) == 0 {
			break
		}
	}
}

func (this *Delete) Init(d registry.Key, zkc ZK) watch {
	this.Delete = d.(registry.Delete)
	this.base.Init(zkc)
	return this
}

func (this *Create) Init(e registry.Key, zkc ZK) watch {
	this.Create = e.(registry.Create)
	this.base.Init(zkc)
	return this
}

func (this *Change) Init(c registry.Key, zkc ZK) watch {
	this.Change = c.(registry.Change)
	this.base.Init(zkc)
	return this
}

func (this *Members) Init(m registry.Key, zkc ZK) watch {
	this.Members = m.(registry.Members)
	this.base.Init(zkc)
	return this
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
		this.stop, err = node.Watch(handler)
		if err != nil {
			return
		}
		if this.timer != nil {
			this.timer.Reset(this.timeout)
		}
	case err == ErrNotExist:
		this.stop, err = this.zk.Watch(path.Path(), handler)
		if err != nil {
			return
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
			}
		}
	}
	if _, err := this.base.watch(this.Delete, f); err != nil {
		return err
	} else {
		return nil
	}
}

func (this *Create) Apply(handler func(k registry.Key, before, after *Node) bool) error {
	f := func(e Event) {
		switch e.Type {
		case zk.EventNodeCreated:
			after, err := this.zk.Get(this.Create.Path())
			if err != nil {
				this.error <- err
			}
			if handler(*this, nil, after) {
				this.base.notify(this)
			}
		}
	}
	if _, err := this.base.watch(this.Create, f); err != nil {
		return err
	} else {
		return nil
	}
}

func (this *Change) Apply(handler func(k registry.Key, before, after *Node) bool) error {
	f := func(e Event) {
		switch e.Type {
		case zk.EventNodeDataChanged:
			after, err := this.zk.Get(this.Path())
			if err != nil {
				this.error <- err
			}
			if handler(*this, this.before, after) {
				this.base.notify(this)
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
				this.error <- err
			}
			if handler(*this, this.before, after) {
				this.base.notify(this)
			}
		}
	}

	if n, err := this.base.watch(this.Top, f); err != nil {
		return err
	} else {
		this.before = n
		return nil
	}
}
