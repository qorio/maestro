package zk

import (
	"encoding/json"
	"errors"
	"github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ZkHosts() []string {
	servers := []string{"localhost:2181"}
	list := os.Getenv("ZK_HOSTS")
	if len(list) > 0 {
		servers = strings.Split(list, ",")
	}
	glog.Infoln("ZK_HOSTS:", servers)
	return servers
}

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
	conn    *zk.Conn
	servers []string
	timeout time.Duration
	events  chan Event

	ephemeral        map[string][]byte
	ephemeral_add    chan *Node
	ephemeral_remove chan string

	retry      chan *kv
	retry_stop chan int
	stop       chan int

	running bool

	watch_stops_chan chan chan bool
	watch_stops      map[chan bool]bool

	shutdown chan int
}

type kv struct {
	key   string
	value []byte
}

type Event struct {
	zk.Event
	Action string
	Note   string
}

func (e Event) AsMap() map[string]interface{} {
	if e.Action != "" {
		return map[string]interface{}{
			"type":   e.Action,
			"note":   e.Note,
			"path":   e.Path,
			"error":  e.Err,
			"server": e.Server,
		}
	} else {
		return map[string]interface{}{
			"type":   event_types[e.Type],
			"state":  states[e.State],
			"path":   e.Path,
			"error":  e.Err,
			"server": e.Server,
		}
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
	for k, v := range this.ephemeral {
		this.retry <- &kv{key: k, value: v}
	}
}

// ephemeral flag here is user requested.
func (this *zookeeper) track_ephemeral(zn *Node, ephemeral bool) {
	if ephemeral || (zn.Stats != nil && zn.Stats.EphemeralOwner > 0) {
		this.ephemeral_add <- zn
	}
}

func (this *zookeeper) untrack_ephemeral(path string) {
	this.ephemeral_remove <- path
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
		ephemeral:        map[string][]byte{},
		ephemeral_add:    make(chan *Node),
		ephemeral_remove: make(chan string),
		retry:            make(chan *kv),
		retry_stop:       make(chan int),
		watch_stops:      make(map[chan bool]bool),
		watch_stops_chan: make(chan chan bool),
		shutdown:         make(chan int),
	}

	go func() {
		<-zz.shutdown
		zz.do_shutdown()
		glog.Infoln("Shutdown complete.")
	}()

	go func() {
		defer glog.Infoln("ZK watcher cache stopped.")
		for {
			watch_stop, open := <-zz.watch_stops_chan
			if !open {
				return
			}
			zz.watch_stops[watch_stop] = true
		}
	}()
	go func() {
		defer glog.Infoln("ZK ephemeral cache stopped.")
		for {
			select {
			case add, open := <-zz.ephemeral_add:
				if !open {
					return
				}
				zz.ephemeral[add.Path] = add.Value
				glog.Infoln("EPHEMERAL-CACHE-ADD: Path=", add.Path, "Value=", string(add.Value))

			case remove, open := <-zz.ephemeral_remove:
				if !open {
					return
				}
				if _, has := zz.ephemeral[remove]; has {
					delete(zz.ephemeral, remove)
					glog.Infoln("EPHEMERAL-CACHE-REMOVE: Path=", remove)
				}
			}
		}
	}()
	go func() {
		defer glog.Infoln("ZK event loop stopped")
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
				zz.events <- Event{Event: evt}
			case <-zz.stop:
				return
			}
		}
	}()
	go func() {
		defer glog.Infoln("ZK ephemeral retry loop stopped")
		for {
			select {
			case r := <-zz.retry:
				if r != nil {
					_, err := zz.CreateEphemeral(r.key, r.value)
					switch err {
					case nil, ErrNodeExists:
						glog.Infoln("EPHEMERAL-RETRY: Key=", r.key, "retry ok.")
						zz.events <- Event{Event: zk.Event{Path: r.key}, Action: "Ephemeral-Retry", Note: "retry ok"}
					default:
						glog.Infoln("EPHEMERAL-RETRY: Key=", r.key, "Err=", err, "retrying.")
						go func() {
							// Non-blocking send from another thread/goroutine
							glog.Warningln("EPHEMERAL-RETRY:", r.key, "resubmit")
							zz.retry <- r
							glog.Warningln("EPHEMERAL-RETRY:", r.key, "submitted")
							zz.events <- Event{Event: zk.Event{Path: r.key}, Action: "Ephemeral-Retry", Note: "retrying"}
						}()
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
	this.shutdown <- 1
	// wait for a close
	<-this.shutdown
	return nil
}

func (this *zookeeper) do_shutdown() {
	glog.Infoln("Shutting down...")

	close(this.ephemeral_add)
	close(this.ephemeral_remove)

	close(this.stop)
	close(this.retry_stop)

	for w, _ := range this.watch_stops {
		close(w)
	}
	close(this.watch_stops_chan)

	this.conn.Close()
	this.conn = nil

	close(this.shutdown)
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
					more = f(Event{Event: event})
				}
				if more {
					// Retry loop
					for {
						glog.Infoln("WATCH-RETRY: Trying to set watch on", path)
						_, _, event_chan, err = this.conn.ExistsW(path)
						if err == nil {
							glog.Infoln("WATCH-RETRY: Continue watching", path)
							this.events <- Event{Event: zk.Event{Path: path}, Action: "Watch-Retry", Note: "retry ok"}
							break
						} else {
							glog.Warningln("WATCH-RETRY: Error -", path, err)
							for _, a := range alerts {
								a(err)
							}
							// Wait a little
							time.Sleep(1 * time.Second)
							glog.Infoln("WATCH-RETRY: Finished waiting. Try again to watch", path)
							this.events <- Event{Event: zk.Event{Path: path}, Action: "Watch-Retry", Note: "retrying"}
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
			f(Event{Event: event})
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
