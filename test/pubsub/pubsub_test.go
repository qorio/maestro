package pubsub

import (
	"fmt"
	_ "github.com/qorio/maestro/pkg/mqtt"
	. "github.com/qorio/maestro/pkg/pubsub"
	. "gopkg.in/check.v1"
	"testing"
)

func TestPubSub(t *testing.T) { TestingT(t) }

type PubSubTests struct{}

var _ = Suite(&PubSubTests{})

func (suite *PubSubTests) TestTopic(c *C) {

	var t Topic
	t = Topic("mqtt://localhost:1235/foo/bar")
	c.Assert(t.Valid(), Equals, true)
	c.Assert(t.Protocol(), Equals, "mqtt")
	c.Assert(t.Path(), Equals, "/foo/bar")

	t = Topic("mqtt://foo/bar")
	c.Assert(t.Valid(), Equals, false)
	t = Topic("mqt://foo/bar")
	c.Assert(t.Valid(), Equals, false)
	t = Topic("mqt://")
	c.Assert(t.Valid(), Equals, false)

	t = Topic("mqtt://localhost:1245/foo/bar")
	c.Assert(t.Path(), Equals, "/foo/bar")

	t = Topic("mqtt://io1-2.internet.org:1234/foo/bar")
	c.Assert(t.Valid(), Equals, true)
	c.Assert(t.Path(), Equals, "/foo/bar")

}

func (suite *PubSubTests) TestBroker(c *C) {

	var b Broker
	b = Broker("mqtt://localhost:1281/path")
	c.Assert(b.Valid(), Equals, false)

	b = Broker("mqtt://iot.eclipse.org:1281")
	c.Assert(b.Valid(), Equals, true)
	c.Assert(b.HostPort(), Equals, "iot.eclipse.org:1281")
	c.Assert(b.Protocol(), Equals, "mqtt")

	t := Topic("mqtt://iot.eclipse-2.org:1281/this/topic")
	c.Assert(t.Valid(), Equals, true)
	c.Assert(t.Broker().Valid(), Equals, true)
	c.Assert(t.Broker().Protocol(), Equals, "mqtt")

	b = Broker("http://localhost:1281")
	c.Assert(b.Valid(), Equals, false)

	b = Broker("mqtt://localhost:1281")
	c.Assert(b.Valid(), Equals, true)

	t = b.Topic("/this/is/topic")
	c.Assert(t.Path(), Equals, "/this/is/topic")
}

func (suite *PubSubTests) TestReader(c *C) {

	t := Topic("mqtt://iot.eclipse.org:1883/reader/test")
	c.Log("protocol=", t.Protocol())
	c.Log("protocol=", t.Broker().Protocol())

	sub, err := t.Broker().PubSub("test")
	c.Assert(err, Equals, nil)

	reader := GetReader(t, sub)
	c.Assert(reader, Not(Equals), nil)

	messages := 5
	done := make(chan bool)
	go func() {
		l := new(int)
		*l = messages
		for {
			buff := make([]byte, 1024)
			n, err := reader.Read(buff)
			c.Assert(err, Equals, nil)
			c.Log(*l, " Read", string(buff[0:n]))
			*l += -1
			if *l == 0 {
				done <- true
				break
			}
		}
	}()
	c.Log("Reader started")

	writer := GetWriter(t, sub)
	c.Assert(writer, Not(Equals), nil)

	for i := 0; i < messages; i++ {
		n, err := writer.Write([]byte(fmt.Sprintf("test-%d", i)))
		c.Assert(n, Not(Equals), 0)
		c.Assert(err, Equals, nil)
		c.Log(i, " Sent")
	}

	c.Log("Waiting for reader to finish.")
	<-done

}
