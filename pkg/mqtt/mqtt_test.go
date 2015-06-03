package mqtt

import (
	"fmt"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func TestMqtt(t *testing.T) { TestingT(t) }

type MqttTests struct{}

var _ = Suite(&MqttTests{})

var (
	local_endpoint = "iot.eclipse.org:1883" //"192.168.59.103:1883"
	topic          = "/this-is-a-test"
)

func (suite *MqttTests) TestConnectDisconnect(c *C) {
	cl, err := Connect("test", local_endpoint, "/test1")
	c.Assert(err, Equals, nil)
	c.Log("Got client=", cl)

	cl.Close()
}

func (suite *MqttTests) TestPublishSubscribe1(c *C) {
	total := 20
	pub, err := Connect("publisher", local_endpoint, topic)
	c.Assert(err, Equals, nil)
	c.Log("Got client=", pub)

	var count = new(int)

	go func() {
		for m := 0; ; m++ {
			message := fmt.Sprintf("message-%d", m)
			pub.Publish([]byte(message))
			if *count == total {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		c.Log("Done publishing")
	}()

	sub, err := Connect("subscriber", local_endpoint, topic)
	receive, err := sub.Subscribe()
	c.Assert(err, Equals, nil)

	for {
		r := <-receive
		c.Log("Received:", string(r))
		*count = *count + 1
		if *count == total {
			break
		}
	}
	c.Log("Finished")
}
