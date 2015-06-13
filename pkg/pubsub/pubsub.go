package pubsub

import (
	"bytes"
	"errors"
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
	sub   Subscriber
	topic Topic
	read  <-chan []byte
	buff  bytes.Buffer
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
	}
	go func() { r.loop() }()

	return &r
}

func (this *reader) loop() {
	for {
		_, err := this.buff.Write(<-this.read)
		if err != nil {
			break
		}
	}
}

func (this *reader) Read(p []byte) (n int, err error) {
	return this.buff.Read(p)
}
