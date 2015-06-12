package pubsub

import (
	"strings"
)

type Topic string

func (t Topic) String() string {
	if i := strings.Index(string(t), "://"); i > 0 {
		return string(t)[i+3:]
	}
	return string(t)
}

func (t Topic) Protocol(protocol string) bool {
	return strings.Index(string(t), protocol+"://") >= 0
}

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

func AsWriter(topic Topic, pub Publisher) *writer {
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
