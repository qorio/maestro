package zk

import (
	"encoding/json"
	"errors"
	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotConnected   = errors.New("zk-not-initialized")
	ErrConflict       = errors.New("error-conflict")
	ErrZkDisconnected = errors.New("error-zk-disconnected")

	ErrNotExist                = zk.ErrNoNode
	ErrConnectionClosed        = zk.ErrConnectionClosed
	ErrUnknown                 = zk.ErrUnknown
	ErrAPIError                = zk.ErrAPIError
	ErrNoAuth                  = zk.ErrNoAuth
	ErrBadVersion              = zk.ErrBadVersion
	ErrNoChildrenForEphemerals = zk.ErrNoChildrenForEphemerals
	ErrNodeExists              = zk.ErrNodeExists
	ErrNotEmpty                = zk.ErrNotEmpty
	ErrSessionExpired          = zk.ErrSessionExpired
	ErrInvalidACL              = zk.ErrInvalidACL
	ErrAuthFailed              = zk.ErrAuthFailed
	ErrClosing                 = zk.ErrClosing
	ErrNothing                 = zk.ErrNothing
	ErrSessionMoved            = zk.ErrSessionMoved

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
	conn           *zk.Conn
	servers        []string
	timeout        time.Duration
	events         chan Event
	ephemeralNodes map[string][]byte

	retry      chan *kv
	retry_stop chan int
	stop       chan int
	lock       sync.RWMutex
	running    bool

	watch_stops_chan chan chan bool
	watch_stops      map[chan bool]bool
}

type kv struct {
	key   string
	value []byte
}

type Event zk.Event

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

func (this *zookeeper) on_disconnect() {
	glog.Warningln("ZK disconnected")
}

func (this *zookeeper) on_connect() {
	for k, v := range this.ephemeralNodes {
		this.retry <- &kv{key: k, value: v}
	}
}

// ephemeral flag here is user requested.
func (this *zookeeper) track_ephemeral(zn *Node, ephemeral bool) {
	if ephemeral || (zn.Stats != nil && zn.Stats.EphemeralOwner > 0) {
		this.lock.Lock()
		defer this.lock.Unlock()
		this.ephemeralNodes[zn.Path] = zn.Value
		glog.Infoln("EPHEMERAL-CACHE: Path=", zn.Path, "Value=", string(zn.Value))
	}
}

func (this *zookeeper) untrack_ephemeral(path string) {
	this.lock.Lock()
	defer this.lock.Unlock()
	delete(this.ephemeralNodes, path)
}

func Connect(servers []string, timeout time.Duration) (*zookeeper, error) {
	conn, events, err := zk.Connect(servers, timeout)
	if err != nil {
		return nil, err
	}
	zz := &zookeeper{
		conn:             conn,
		servers:          servers,
		timeout:          timeout,
		events:           make(chan Event),
		stop:             make(chan int),
		ephemeralNodes:   map[string][]byte{},
		retry:            make(chan *kv),
		retry_stop:       make(chan int),
		watch_stops:      make(map[chan bool]bool),
		watch_stops_chan: make(chan chan bool),
	}
	go func() {
		for {
			watch_stop, open := <-zz.watch_stops_chan
			if !open {
				return
			}
			zz.watch_stops[watch_stop] = true
		}
	}()
	go func() {
		for {
			select {
			case evt := <-events:
				glog.Infoln("ZK-Event-Main:", evt)
				switch evt.State {
				case StateExpired:
					glog.Warningln("ZK state expired --> sent by server on reconnection.")
					zz.on_connect()
				case StateHasSession:
					glog.Warningln("ZK state has-session")
					zz.on_connect()
				case StateDisconnected:
					glog.Warningln("ZK state disconnected")
					zz.on_disconnect()
				}
				zz.events <- Event(evt)
			case <-zz.stop:
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case r := <-zz.retry:
				if r != nil {
					_, err := zz.CreateEphemeral(r.key, r.value)
					switch err {
					case nil, ErrNodeExists:
						glog.Infoln("EPHEMERAL-RETRY: key=", r.key, "value=", string(r.value), "reset ok.")
					default:
						glog.Infoln("EPHEMERAL-RETRY: key=", r.key, "value=", string(r.value), "Err=", err, "retrying.")
						// Non-blocking send
						select {
						case zz.retry <- r:
							glog.Warningln("EPHEMERAL-RETRY:", r)
						}
					}
				}
			case <-zz.retry_stop:
				return
			}
		}
	}()

	glog.Infoln("Connected to zk:", servers)
	return zz, nil
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
	glog.Infoln("Shutting down...")
	close(this.stop)
	close(this.retry_stop)

	for w, _ := range this.watch_stops {
		close(w)
	}
	close(this.watch_stops_chan)

	this.conn.Close()
	this.conn = nil
	glog.Infoln("Finished shutting down")
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
	stop := make(chan bool)
	this.watch_stops_chan <- stop
	go func() {
		for {
			select {
			case event := <-event_chan:

				more := true

				glog.Infoln("WATCH: State change. Path=", path, "State=", event.State)
				switch event.State {
				case zk.StateExpired:
					for _, a := range alerts {
						a(ErrSessionExpired)
					}
				case zk.StateDisconnected:
					for _, a := range alerts {
						a(ErrZkDisconnected)
					}
				default:
					more = f(Event(event))
				}
				if more {
					// Retry loop
					for {
						glog.Infoln("WATCH: Trying to set watch on", path)
						_, _, event_chan, err = this.conn.ExistsW(path)
						if err == nil {
							glog.Infoln("WATCH: Continue watching", path)
							break
						} else {
							glog.Warningln("WATCH: Error -", path, err)
							for _, a := range alerts {
								a(err)
							}
							// Wait a little
							time.Sleep(1 * time.Second)
							glog.Infoln("WATCH: Finished waiting. Try again to watch", path)
						}
					}
				}

			case <-stop:
				glog.Infoln("WATCH: Watch terminated:", path)
				return
			}
		}
	}()
	glog.Infoln("WATCH: Started watch on", path)
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

func (this *zookeeper) Delete(path string) error {
	if err := this.check(); err != nil {
		return err
	}
	this.untrack_ephemeral(path)
	return this.conn.Delete(path, -1)
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

func filter_err(err error) error {
	switch {
	case err == zk.ErrNoNode:
		return ErrNotExist
	default:
		return err
	}
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
	if ephemeral {
		glog.Infoln("EPHEMERAL: created Path=", key, "Value=", string(value))
	}
	zn := &Node{Path: p, Value: value, zk: this}
	this.track_ephemeral(zn, ephemeral)

	return this.Get(p)
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
