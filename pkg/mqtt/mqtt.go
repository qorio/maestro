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
	BrokerAddr     string        `json:"broker_addr"`
	ClientId       string        `json:"client_id"`
	QoS            byte          `json:"qos"` //MQTT.QoS
	Topic          string        `json:"topic"`
	PublishTimeout time.Duration `json:"publish_timeout"`
	client         *MQTT.Client
}

func (this *Client) Write(p []byte) (n int, err error) {
	n = len(p)
	this.Publish(p)
	return n, nil
}

func NewClient(id, addr, topic string) (*Client, error) {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://" + addr)
	opts.SetClientID(id)
	c := MQTT.NewClient(opts)
	wait := c.Connect()
	error := wait.Wait()
	if error {
		return nil, errors.New("cannot-connect")
	}
	return &Client{
		QoS:            QOS_ZERO,
		BrokerAddr:     addr,
		ClientId:       id,
		client:         c,
		Topic:          topic,
		PublishTimeout: time.Second * 1,
	}, nil
}

func (this *Client) Publish(message []byte) {
	token := this.client.Publish(this.Topic, this.QoS, false, message)
	token.WaitTimeout(this.PublishTimeout)
}
