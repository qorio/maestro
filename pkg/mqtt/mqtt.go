package mqtt

import (
	"errors"
	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"github.com/qorio/maestro/pkg/pubsub"
	"time"
)

var (
	ErrNotSupportedProtocol = errors.New("not-supported-protocol")
	ErrConnect              = errors.New("error-connect")
)

const (
	QOS_ZERO = 0 //MQTT.QoS = MQTT.QOS_ZERO
	QOS_ONE  = 1 //MQTT.QoS = MQTT.QOS_ONE
	QOS_TWO  = 2 //MQTT.QoS = MQTT.QOS_TWO
)

func init() {
	pubsub.Register("mqtt", func(id, addr string, options ...interface{}) (pubsub.PubSub, error) {
		return Connect(id, addr, options...)
	})
}

type Client struct {
	BrokerAddr       string        `json:"broker_addr"`
	ClientId         string        `json:"client_id"`
	QoS              byte          `json:"qos"` //MQTT.QoS
	PublishTimeout   time.Duration `json:"publish_timeout"`
	SubscribeTimeout time.Duration `json:"subscribe_timeout"`
	client           *MQTT.Client
}

func Connect(id, addr string, options ...interface{}) (pubsub.PubSub, error) {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://" + addr)
	opts.SetClientID(id)
	c := MQTT.NewClient(opts)
	wait := c.Connect()
	ready := wait.Wait()
	if !ready {
		return nil, ErrConnect
	}
	return &Client{
		QoS:              QOS_ZERO,
		BrokerAddr:       addr,
		ClientId:         id,
		client:           c,
		PublishTimeout:   time.Second * 1,
		SubscribeTimeout: time.Second * 1,
	}, nil
}

func errNotSupportedProtocol(t pubsub.Topic) error {
	return errors.New("not-supported-protocol:" + t.String())
}

func (this *Client) Publish(topic pubsub.Topic, message []byte) error {
	if "mqtt" != topic.Protocol() {
		return errNotSupportedProtocol(topic)
	}
	token := this.client.Publish(topic.Path(), this.QoS, false, message)
	token.WaitTimeout(this.PublishTimeout)
	if token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (this *Client) Subscribe(topic pubsub.Topic) (<-chan []byte, error) {
	if "mqtt" != topic.Protocol() {
		return nil, errNotSupportedProtocol(topic)
	}
	out := make(chan []byte)
	token := this.client.Subscribe(topic.Path(), this.QoS, func(cl *MQTT.Client, m MQTT.Message) {
		out <- m.Payload()
	})
	token.WaitTimeout(this.SubscribeTimeout)
	if token.Error() != nil {
		return nil, token.Error()
	}
	return out, nil
}

func (this *Client) Close() {
	this.client.Disconnect(250)
}
