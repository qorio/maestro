package mqtt

import (
	"errors"
	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"time"
)

const (
	QOS_ZERO = 0 //MQTT.QoS = MQTT.QOS_ZERO
	QOS_ONE  = 1 //MQTT.QoS = MQTT.QOS_ONE
	QOS_TWO  = 2 //MQTT.QoS = MQTT.QOS_TWO
)

type Client struct {
	BrokerAddr       string        `json:"broker_addr"`
	ClientId         string        `json:"client_id"`
	QoS              byte          `json:"qos"` //MQTT.QoS
	Topic            string        `json:"topic"`
	PublishTimeout   time.Duration `json:"publish_timeout"`
	SubscribeTimeout time.Duration `json:"subscribe_timeout"`
	client           *MQTT.Client
}

func (this *Client) Write(p []byte) (n int, err error) {
	n = len(p)
	this.Publish(p)
	return n, nil
}

func Connect(id, addr, topic string) (*Client, error) {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://" + addr)
	opts.SetClientID(id)
	c := MQTT.NewClient(opts)
	wait := c.Connect()
	ready := wait.Wait()
	if !ready {
		return nil, errors.New("cannot-connect")
	}
	return &Client{
		QoS:              QOS_ZERO,
		BrokerAddr:       addr,
		ClientId:         id,
		client:           c,
		Topic:            topic,
		PublishTimeout:   time.Second * 1,
		SubscribeTimeout: time.Second * 1,
	}, nil
}

func (this *Client) Publish(message []byte) error {
	token := this.client.Publish(this.Topic, this.QoS, false, message)
	token.WaitTimeout(this.PublishTimeout)
	if token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (this *Client) Subscribe() (<-chan []byte, error) {
	out := make(chan []byte)
	token := this.client.Subscribe(this.Topic, this.QoS, func(cl *MQTT.Client, m MQTT.Message) {
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
