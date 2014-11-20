package zk

import (
	"errors"
	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotConnected = errors.New("zk-not-connected")
	ErrNotExist     = errors.New("zk-not-exist")
)

type zookeeper struct {
	conn    *zk.Conn
	servers []string
	timeout time.Duration
}

func Connect(servers []string, timeout time.Duration) (*zookeeper, error) {
	conn, _, err := zk.Connect(servers, timeout)
	if err != nil {
		return nil, err
	}
	glog.Infoln("Connected to zk:", servers)
	return &zookeeper{
		conn:    conn,
		servers: servers,
		timeout: timeout,
	}, nil
}

func (this *zookeeper) check() error {
	if this.conn == nil {
		return ErrNotConnected
	}
	return nil
}

func (this *zookeeper) Close() error {
	this.conn.Close()
	this.conn = nil
	return nil
}

func (this *zookeeper) Reconnect() error {
	p, err := Connect(this.servers, this.timeout)
	if err != nil {
		return err
	} else {
		this = p
		return nil
	}
}

func (this *zookeeper) Delete(path string) error {
	if err := this.check(); err != nil {
		return err
	}
	return this.conn.Delete(path, -1)
}

func (this *zookeeper) Get(path string) (*znode, error) {
	if err := this.check(); err != nil {
		return nil, err
	}

	exists, _, err := this.conn.Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotExist
	}
	value, stats, err := this.conn.Get(path)
	if err != nil {
		return nil, err
	}
	return &znode{Path: path, Value: value, Stats: stats, zk: this}, nil
}

func (this *zookeeper) Watch(path string, f func(zk.Event)) (chan<- bool, error) {
	if err := this.check(); err != nil {
		return nil, err
	}
	_, _, event_chan, err := this.conn.ExistsW(path)
	if err != nil {
		return nil, err
	}
	return run_watch(f, event_chan)
}

func (this *zookeeper) Create(path string, value []byte) (*znode, error) {
	if err := this.check(); err != nil {
		return nil, err
	}
	if err := this.build_parents(path); err != nil {
		return nil, err
	}
	return this.create(path, value, false)
}

func (this *zookeeper) CreateEphemeral(path string, value []byte) (*znode, error) {
	if err := this.check(); err != nil {
		return nil, err
	}
	if err := this.build_parents(path); err != nil {
		return nil, err
	}
	return this.create(path, value, true)
}

func get_targets(path string) []string {
	p := path
	if p[0:1] != "/" {
		p = "/" + path // Must begin with /
	}
	pp := strings.Split(p, "/")
	t := []string{}
	root := ""
	for _, x := range pp[1:] {
		z := root + "/" + x
		root = z
		t = append(t, z)
	}
	return t
}

func (this *zookeeper) build_parents(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	for _, p := range get_targets(dir) {
		exists, _, err := this.conn.Exists(p)
		if err != nil {
			return err
		}
		if !exists {
			_, err := this.create(p, []byte{}, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (this *zookeeper) create(path string, value []byte, ephemeral bool) (*znode, error) {
	key := path
	flags := int32(0)
	if ephemeral {
		flags = int32(zk.FlagEphemeral)
	}
	acl := zk.WorldACL(zk.PermAll)
	p, err := this.conn.Create(key, value, flags, acl)
	if err != nil {
		return nil, err
	}
	zn := &znode{Path: p, Value: value, zk: this}
	err = zn.Get()
	if err != nil {
		return nil, err
	}
	return zn, nil
}

type znode struct {
	Path  string
	Value []byte
	Stats *zk.Stat
	zk    *zookeeper
}

func filter_err(err error) error {
	switch {
	case err == zk.ErrNoNode:
		return ErrNotExist
	default:
		return err
	}
}

func (this *znode) Get() error {
	if err := this.zk.check(); err != nil {
		return err
	}
	v, s, err := this.zk.conn.Get(this.Path)
	if err != nil {
		return filter_err(err)
	}
	this.Value = v
	this.Stats = s
	return nil
}

func run_watch(f func(zk.Event), event_chan <-chan zk.Event) (chan<- bool, error) {
	if f != nil {
		stop := make(chan bool, 1)
		go func() {
			// Note ZK only fires once and after that we need to reschedule.
			// With this api this may mean we get a new event channel.
			// Therefore, there's no point looping in here for more than 1 event.
			select {
			case event := <-event_chan:
				f(event)
				// TODO - reschedule another watch
			case b := <-stop:
				if b {
					glog.Infoln("Watch terminated")
					return
				}
			}
		}()
		return stop, nil
	}
	return nil, nil
}

func (this *znode) Watch(f func(zk.Event)) (chan<- bool, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	value, stat, event_chan, err := this.zk.conn.GetW(this.Path)
	if err != nil {
		return nil, filter_err(err)
	}
	this.Value = value
	this.Stats = stat
	return run_watch(f, event_chan)
}

func (this *znode) Set(value []byte) error {
	if err := this.zk.check(); err != nil {
		return err
	}
	s, err := this.zk.conn.Set(this.Path, value, this.Stats.Version)
	if err != nil {
		return filter_err(err)
	}
	this.Value = value
	this.Stats = s
	return nil
}

func (this *znode) CountChildren() int32 {
	if this.Stats == nil {
		if err := this.Get(); err != nil {
			return -1
		}
	}
	return this.Stats.NumChildren
}

func (this *znode) Children() ([]*znode, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	paths, s, err := this.zk.conn.Children(this.Path)
	if err != nil {
		return nil, err
	} else {
		this.Stats = s
		children := make([]*znode, len(paths))
		for i, p := range paths {
			children[i] = &znode{Path: this.Path + "/" + p, zk: this.zk}
			err := children[i].Get()
			if err != nil {
				return nil, err
			}
		}
		return children, nil
	}
}

func append_string_slices(a, b []string) []string {
	l := len(a)
	ll := make([]string, l+len(b))
	copy(ll, a)
	for i, n := range b {
		ll[i+l] = n
	}
	return ll
}

func append_znode_slices(a, b []*znode) []*znode {
	l := len(a)
	ll := make([]*znode, l+len(b))
	copy(ll, a)
	for i, n := range b {
		ll[i+l] = n
	}
	return ll
}

func (this *znode) ListAllRecursive() ([]string, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]string, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}
	for _, n := range children {
		l, err := n.ListAllRecursive()
		if err != nil {
			return nil, err
		}
		list = append_string_slices(list, l)
		list = append(list, n.Path)
	}
	return list, nil
}

func (this *znode) ChildrenRecursive() ([]*znode, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*znode, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}
	for _, n := range children {
		l, err := n.ChildrenRecursive()
		if err != nil {
			return nil, err
		}
		list = append_znode_slices(list, l)
		list = append(list, n)
	}
	return list, nil
}

func (this *znode) Delete() error {
	if err := this.zk.check(); err != nil {
		return err
	}
	err := this.zk.Delete(this.Path)
	if err != nil {
		return err
	} else {
		return nil
	}
}
