package pubsub

import (
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
