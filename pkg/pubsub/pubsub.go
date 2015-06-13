package pubsub

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
)

const (
	TopicSyntax  = "(?P<protocol>mqtt|kfka)://(?P<host>[a-z0-9A-Z\\-]+(\\.[a-z0-9A-Z\\-]+)*)(:*(?P<port>[0-9]+))(?P<path>/.*)"
	BrokerSyntax = "(?P<protocol>mqtt|kfka)://(?P<host>[a-z0-9A-Z\\-]+(\\.[a-z0-9A-Z\\-]+)*)(:*(?P<port>[0-9]+))$"
)

var (
	ErrNotSupported = errors.New("not-supported")

	TopicRegex  = regexp.MustCompile(TopicSyntax)
	BrokerRegex = regexp.MustCompile(BrokerSyntax)

	factories = map[string]Factory{}

	// By address. This allows for reuse of connections
	lock    sync.Mutex
	clients = map[string]PubSub{}
)

type Publisher interface {
	Publish(topic Topic, message []byte) error
}

type Subscriber interface {
	Subscribe(topic Topic) (<-chan []byte, error)
}

type PubSub interface {
	Publisher
	Subscriber
	Close()
}

type Factory func(id, addr string, options ...interface{}) (PubSub, error)

func Register(protocol string, factory Factory) {
	factories[protocol] = factory
}

type Broker string

func (b Broker) PubSub(id string, options ...interface{}) (PubSub, error) {
	lock.Lock()
	defer lock.Unlock()
	f, has := factories[b.Protocol()]

	if !has {
		return nil, ErrNotSupported
	}
	k := b.HostPort() + "/" + id
	if _, has := clients[k]; !has {
		c, err := f(id, b.HostPort(), options...)
		if err != nil {
			return nil, err
		} else {
			clients[k] = c
		}
	}
	return clients[k], nil
}

func (b Broker) Valid() bool {
	return BrokerRegex.MatchString(string(b))
}

func (b Broker) Protocol() string {
	return BrokerRegex.ReplaceAllString(string(b), "${protocol}")
}

func (b Broker) HostPort() string {
	return BrokerRegex.ReplaceAllString(string(b), "${host}:${port}")
}

func (b Broker) Topic(path string) Topic {
	return Topic(string(b) + path)
}

func (b Broker) String() string {
	return string(b)
}

type Topic string

func (t Topic) Broker() Broker {
	return Broker(TopicRegex.ReplaceAllString(string(t), "${protocol}://${host}:${port}"))
}

func (t Topic) Valid() bool {
	return TopicRegex.MatchString(string(t))
}

func (t Topic) Protocol() string {
	return TopicRegex.ReplaceAllString(string(t), "${protocol}")
}

func (t Topic) HostPort() string {
	return TopicRegex.ReplaceAllString(string(t), "${host}:${port}")
}

func (t Topic) Path() string {
	return TopicRegex.ReplaceAllString(string(t), "${path}")
}

func (t Topic) String() string {
	return string(t)
}

type writer struct {
	pub   Publisher
	topic Topic
}

func GetWriter(topic Topic, pub Publisher) io.Writer {
	return &writer{
		topic: topic,
		pub:   pub,
	}
}

func (this *writer) Write(p []byte) (n int, err error) {
	n = len(p)
	this.pub.Publish(this.topic, p)
	return n, nil
}

type reader struct {
	sub        Subscriber
	topic      Topic
	read       <-chan []byte
	buff       bytes.Buffer
	ready      chan bool
	bytes_read int
}

func GetReader(topic Topic, sub Subscriber) io.Reader {
	read, err := sub.Subscribe(topic)
	if err != nil {
		return nil
	}

	r := reader{
		topic: topic,
		sub:   sub,
		read:  read,
		ready: make(chan bool),
	}
	go func() { r.loop() }()

	return &r
}

func (this *reader) loop() {
	for {
		m := <-this.read
		fmt.Printf("TOPIC %s -- %s (%d)\n", this.topic, string(m), len(m))
		_, err := this.buff.Write(m)
		this.bytes_read += len(m)
		this.ready <- err == nil
		if err != nil {
			break
		}
	}
}

// TODO - implement some kind of session / flow control because
// 1. the topic never goes away; however there may be no messages published.
// 2. when no messages are published, there's no data in the buffer so the
//    read will get a EOF.  This will cause the reader to throw a EOF and
//    possbily terminating the something down stream (like a /bin/bash process).
func (this *reader) Read(p []byte) (n int, err error) {
	if this.buff.Len() == 0 {
		ready := <-this.ready
		if !ready {
			return 0, io.EOF
		}
	}
	n, err = this.buff.Read(p)
	fmt.Printf("READER %s -- %s (%d), err=%s\n", this.topic, string(p), n, err)
	return
}
