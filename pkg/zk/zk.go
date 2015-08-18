package zk

import (
	"encoding/json"
	"errors"
	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotConnected   = errors.New("zk-not-initialized")
	ErrNotExist       = zk.ErrNoNode
	ErrConflict       = errors.New("error-conflict")
	ErrZkDisconnected = errors.New("error-zk-disconnected")
)

const (
	StateUnknown           = zk.StateUnknown
	StateDisconnected      = zk.StateDisconnected
	StateConnecting        = zk.StateConnecting
	StateAuthFailed        = zk.StateAuthFailed
	StateConnectedReadOnly = zk.StateConnectedReadOnly
	StateSaslAuthenticated = zk.StateSaslAuthenticated
	StateExpired           = zk.StateExpired
	StateConnected         = zk.StateConnected
	StateHasSession        = zk.StateHasSession
)

type Event zk.Event

var (
	event_types = map[zk.EventType]string{
		zk.EventNodeCreated:         "node-created",
		zk.EventNodeDeleted:         "node-deleted",
		zk.EventNodeDataChanged:     "node-data-changed",
		zk.EventNodeChildrenChanged: "node-children-changed",
		zk.EventSession:             "session",
		zk.EventNotWatching:         "not-watching",
	}
	states = map[zk.State]string{
		zk.StateUnknown:           "state-unknown",
		zk.StateDisconnected:      "state-disconnected",
		zk.StateAuthFailed:        "state-auth-failed",
		zk.StateConnectedReadOnly: "state-connected-readonly",
		zk.StateSaslAuthenticated: "state-sasl-authenticated",
		zk.StateExpired:           "state-expired",
		zk.StateConnected:         "state-connected",
		zk.StateHasSession:        "state-has-session",
	}
)

func (e Event) AsMap() map[string]interface{} {
	return map[string]interface{}{
		"type":   event_types[e.Type],
		"state":  states[e.State],
		"path":   e.Path,
		"error":  e.Err,
		"server": e.Server,
	}
}

func (e Event) JSON() string {
	buff, _ := json.Marshal(e.AsMap())
	return string(buff)
}

type ZK interface {
	Reconnect() error
	Close() error
	Events() <-chan Event
	Create(string, []byte) (*Node, error)
	CreateEphemeral(string, []byte) (*Node, error)
	Get(string) (*Node, error)
	Watch(string, func(Event)) (chan<- bool, error)
	WatchChildren(string, func(Event)) (chan<- bool, error)
	KeepWatch(string, func(Event) bool, ...func(error)) (chan<- bool, error)
	Delete(string) error
}

type zookeeper struct {
	conn    *zk.Conn
	servers []string
	timeout time.Duration
	events  <-chan Event
	stop    chan<- bool
}

func (z *Node) GetPath() string {
	return z.Path
}
func (z *Node) GetBasename() string {
	return filepath.Base(z.Path)
}
func (z *Node) GetValue() []byte {
	return z.Value
}
func (z *Node) GetValueString() string {
	return string(z.Value)
}
func (z *Node) IsLeaf() bool {
	return z.Leaf
}

type Node struct {
	Path    string
	Value   []byte
	Members []string
	Stats   *zk.Stat
	Leaf    bool
	zk      *zookeeper
}

func Connect(servers []string, timeout time.Duration) (*zookeeper, error) {
	conn, _events, err := zk.Connect(servers, timeout)
	if err != nil {
		return nil, err
	}

	stop_chan := make(chan bool)
	events := make(chan Event)
	go func() {
		for {
			select {
			case evt := <-_events:
				events <- Event(evt)
			case stop := <-stop_chan:
				if stop {
					break
				}
			}
		}
	}()
	glog.Infoln("Connected to zk:", servers)
	return &zookeeper{
		conn:    conn,
		servers: servers,
		timeout: timeout,
		events:  events,
		stop:    stop_chan,
	}, nil
}

func (this *zookeeper) check() error {
	if this.conn == nil {
		return ErrNotConnected
	}
	return nil
}

func (this *zookeeper) Events() <-chan Event {
	return this.events
}

func (this *zookeeper) Close() error {
	this.conn.Close()
	this.conn = nil
	this.stop <- true
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

func (this *zookeeper) Get(path string) (*Node, error) {
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
	return &Node{Path: path, Value: value, Stats: stats, zk: this}, nil
}

func (this *zookeeper) Watch(path string, f func(Event)) (chan<- bool, error) {
	if err := this.check(); err != nil {
		return nil, err
	}
	_, _, event_chan, err := this.conn.ExistsW(path)
	if err != nil {
		return nil, err
	}
	return run_watch(f, event_chan)
}

func (this *zookeeper) WatchChildren(path string, f func(Event)) (chan<- bool, error) {
	if err := this.check(); err != nil {
		return nil, err
	}

	_, _, event_chan, err := this.conn.ChildrenW(path)
	switch {

	case err == ErrNotExist:
		_, _, event_chan0, err0 := this.conn.ExistsW(path)
		if err0 != nil {
			return nil, err0
		}
		// First watch for creation
		// Use a common stop
		stop1 := make(chan bool)
		_, err1 := run_watch(func(e Event) {
			if e.Type == zk.EventNodeCreated {
				if _, _, event_chan2, err2 := this.conn.ChildrenW(path); err2 == nil {
					// then watch for children
					run_watch(f, event_chan2, stop1)
				}
			}
		}, event_chan0, stop1)
		return stop1, err1

	case err == nil:
		return run_watch(f, event_chan)

	default:
		return nil, err
	}
}

func (this *zookeeper) KeepWatch(path string, f func(Event) bool, alerts ...func(error)) (chan<- bool, error) {
	if err := this.check(); err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errors.New("error-nil-watcher")
	}

	_, _, event_chan, err := this.conn.ExistsW(path)
	if err != nil {
		go func() {
			for _, a := range alerts {
				a(err)
			}
		}()
		return nil, err
	}
	stop := make(chan bool, 1)
	go func() {
		glog.Infoln("Starting watch on:", path)
		for {
			select {
			case event := <-event_chan:

				more := false
				switch event.State {
				case zk.StateDisconnected:
					go func() {
						for _, a := range alerts {
							a(ErrZkDisconnected)
						}
					}()
					more = true
				default:
					more = f(Event(event))
				}
				if more {
					// Retry loop
					for {
						glog.V(100).Infoln("Trying to set watch on", path)
						_, _, event_chan, err = this.conn.ExistsW(path)
						if err == nil {
							glog.Infoln("Continue watching", path)
							break
						} else {
							glog.Warningln("Error on watch", path, err)
							go func() {
								for _, a := range alerts {
									a(err)
								}
							}()
							// Wait a little
							time.Sleep(1 * time.Second)
						}
					}
				}

			case b := <-stop:
				if b {
					glog.Infoln("Watch terminated:", path)
					return
				}
			}
		}
	}()
	return stop, nil
}

func (this *zookeeper) Create(path string, value []byte) (*Node, error) {
	if err := this.check(); err != nil {
		return nil, err
	}
	if err := this.build_parents(path); err != nil {
		return nil, err
	}
	return this.create(path, value, false)
}

func (this *zookeeper) CreateEphemeral(path string, value []byte) (*Node, error) {
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

func (this *zookeeper) create(path string, value []byte, ephemeral bool) (*Node, error) {
	key := path
	flags := int32(0)
	if ephemeral {
		flags = int32(zk.FlagEphemeral)
	}
	acl := zk.WorldACL(zk.PermAll) // TODO - PermAll permission
	p, err := this.conn.Create(key, value, flags, acl)
	if err != nil {
		return nil, err
	}
	zn := &Node{Path: p, Value: value, zk: this}
	err = zn.Get()
	if err != nil {
		return nil, err
	}
	return zn, nil
}

func filter_err(err error) error {
	switch {
	case err == zk.ErrNoNode:
		return ErrNotExist
	default:
		return err
	}
}

func (this *Node) Get() error {
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

func run_watch(f func(Event), event_chan <-chan zk.Event, optionalStop ...chan bool) (chan bool, error) {
	if f == nil {
		return nil, nil
	}

	stop := make(chan bool, 1)
	if len(optionalStop) > 0 {
		stop = optionalStop[0]
	}

	go func() {
		// Note ZK only fires once and after that we need to reschedule.
		// With this api this may mean we get a new event channel.
		// Therefore, there's no point looping in here for more than 1 event.
		select {
		case event := <-event_chan:
			f(Event(event))
		case b := <-stop:
			if b {
				glog.Infoln("Watch terminated")
				return
			}
		}
	}()
	return stop, nil
}

func (this *Node) Watch(f func(Event)) (chan<- bool, error) {
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

func (this *Node) WatchChildren(f func(Event)) (chan<- bool, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	members, stat, event_chan, err := this.zk.conn.ChildrenW(this.Path)
	if err != nil {
		return nil, filter_err(err)
	}
	this.Members = members
	this.Stats = stat
	return run_watch(f, event_chan)
}

func (this *Node) Set(value []byte) error {
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

func (this *Node) CountChildren() int32 {
	if this.Stats == nil {
		if err := this.Get(); err != nil {
			return -1
		}
	}
	return this.Stats.NumChildren
}

func (this *Node) Children() ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	paths, s, err := this.zk.conn.Children(this.Path)
	if err != nil {
		return nil, err
	} else {
		this.Stats = s
		children := make([]*Node, len(paths))
		for i, p := range paths {
			children[i] = &Node{Path: this.Path + "/" + p, zk: this.zk}
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

func append_node_slices(a, b []*Node) []*Node {
	l := len(a)
	ll := make([]*Node, l+len(b))
	copy(ll, a)
	for i, n := range b {
		ll[i+l] = n
	}
	return ll
}

func (this *Node) ListAllRecursive() ([]string, error) {
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

func (this *Node) ChildrenRecursive() ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*Node, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}

	this.Leaf = len(children) == 0

	for _, n := range children {
		l, err := n.ChildrenRecursive()
		if err != nil {
			return nil, err
		}
		list = append_node_slices(list, l)
		list = append(list, n)
	}
	return list, nil
}

// Recursively go through all the children.  Apply filter for each node. If filter returns
// true for the particular node, this node (though not necessarily all its children) will be
// excluded.  This is useful for searching through all true by name or by whether it's a parent
// node or not.
func (this *Node) FilterChildrenRecursive(filter func(*Node) bool) ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*Node, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}

	this.Leaf = len(children) == 0

	for _, n := range children {
		l, err := n.FilterChildrenRecursive(filter)
		if err != nil {
			return nil, err
		}
		list = append_node_slices(list, l)
		add := filter == nil || (filter != nil && !filter(n))
		if add {
			list = append(list, n)
		}
	}
	return list, nil
}

func (this *Node) VisitChildrenRecursive(accept func(*Node) bool) ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*Node, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}

	this.Leaf = len(children) == 0
	for _, n := range children {
		l, err := n.VisitChildrenRecursive(accept)
		if err != nil {
			return nil, err
		}
		list = append_node_slices(list, l)
		if accept == nil || (accept != nil && accept(n)) {
			list = append(list, n)
		}
	}
	return list, nil
}

func (this *Node) Delete() error {
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

func (this *Node) Increment(increment int) (int, error) {
	if err := this.zk.check(); err != nil {
		return -1, err
	}
	count, err := strconv.Atoi(this.GetValueString())
	if err != nil {
		count = 0
	}
	count += increment
	err = this.Set([]byte(strconv.Itoa(count)))
	if err != nil {
		return -1, err
	}
	return count, nil
}

func (this *Node) CheckAndIncrement(current, increment int) (int, error) {
	if err := this.zk.check(); err != nil {
		return -1, err
	}
	count, err := strconv.Atoi(this.GetValueString())
	switch {
	case err != nil:
		return -1, err
	case count != current:
		return -1, ErrConflict
	}
	count += increment
	err = this.Set([]byte(strconv.Itoa(count)))
	if err != nil {
		return -1, err
	}
	return count, nil
}
