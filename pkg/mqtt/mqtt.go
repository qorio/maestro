package mqtt

import (
	"errors"
	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/pubsub"
	"sync"
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

type ClientOptions struct {
	KeepAlive            time.Duration `json:"keep_alive_interval,omitempty"`
	MaxReconnectInterval time.Duration `json:"max_reconnect_interval,omitempty"`
	AutoReconnect        bool          `json:"auto_reconnect"`
}

type Client struct {
	BrokerAddr       string        `json:"broker_addr"`
	ClientId         string        `json:"client_id"`
	QoS              byte          `json:"qos"` //MQTT.QoS
	PublishTimeout   time.Duration `json:"publish_timeout"`
	SubscribeTimeout time.Duration `json:"subscribe_timeout"`
	client           *MQTT.Client
}

var (
	subscribed_topics_by_client_lock sync.Mutex
	subscribed_topics_by_client      = make(map[*MQTT.Client]map[string]chan []byte)
	pubsubclient_by_client           = make(map[*MQTT.Client]*Client)
)

func track_topic(c *Client, topic string, out chan []byte) {
	defer subscribed_topics_by_client_lock.Unlock()
	subscribed_topics_by_client_lock.Lock()

	if _, has := subscribed_topics_by_client[c.client]; !has {
		subscribed_topics_by_client[c.client] = make(map[string]chan []byte)
	}
	subscribed_topics_by_client[c.client][topic] = out
	pubsubclient_by_client[c.client] = c
}

func Connect(id, addr string, options ...interface{}) (pubsub.PubSub, error) {
	opts := MQTT.NewClientOptions().AddBroker("tcp://" + addr).SetClientID(id)
	// some default values
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(10 * time.Minute)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetConnectionLostHandler(func(cl *MQTT.Client, err error) {
		glog.Warningln("MQTT CONNECTION LOST", cl, "Err=", err)
		// TODO - send message over channel
	})
	opts.SetOnConnectHandler(func(cl *MQTT.Client) {
		glog.Infoln("MQTT CONNECTED", cl)
		if set, has := subscribed_topics_by_client[cl]; has {
			for topic, out := range set {
				// Re-subscribe
				qos := pubsubclient_by_client[cl].QoS
				timeout := pubsubclient_by_client[cl].SubscribeTimeout

				glog.Infoln("RESUBSCRIBE", topic, "QoS=", qos)
				token := cl.Subscribe(topic, qos, func(cl *MQTT.Client, m MQTT.Message) {
					out <- m.Payload()
				})
				token.WaitTimeout(timeout)
				if token.Error() != nil {
					glog.Warningln("RE-SUBSCRIBE TIMEOUT", "Topic=", topic, "Client=", cl, "Err=", token.Error())
				}
			}
		}
	})
	var clientOptions *ClientOptions
	if len(options) > 0 {
		switch options[0].(type) {
		case *ClientOptions:
			clientOptions = options[0].(*ClientOptions)
		case ClientOptions:
			copy := options[0].(ClientOptions)
			clientOptions = &copy
		}
		if clientOptions != nil {
			opts.SetAutoReconnect(clientOptions.AutoReconnect)
			if clientOptions.MaxReconnectInterval.Seconds() > 0 {
				opts.SetMaxReconnectInterval(clientOptions.MaxReconnectInterval)
			}
			if clientOptions.KeepAlive.Seconds() > 0 {
				opts.SetKeepAlive(clientOptions.KeepAlive)
			}
		}
	}

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

	track_topic(this, topic.Path(), out)
	return out, nil
}

func (this *Client) Close() {
	this.client.Disconnect(250)
}
