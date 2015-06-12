package pubsub

import (
	"bytes"
	"io"
	"regexp"
)

const (
	TopicSyntax  = "(?P<protocol>mqtt|mqtts|kfka|kfkas)://(?P<host>[a-zA-Z0-9]+(?P<port>:[0-9]+))*(?P<path>/.*)"
	BrokerSyntax = "(?P<protocol>mqtt|mqtts|kfka|kfkas)://(?P<host>[a-zA-Z0-9]+(?P<port>:[0-9]+))*$"
)

var (
	TopicRegex  = regexp.MustCompile(TopicSyntax)
	BrokerRegex = regexp.MustCompile(BrokerSyntax)
)

type Broker string

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

type Topic string

func (t Topic) Broker() Broker {
	return Broker(BrokerRegex.ReplaceAllString(string(t), "${protocol}://${host}:${port}"))
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

// func (t Topic) String() string {
//  	return string(t)
// }

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
