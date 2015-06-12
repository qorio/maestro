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
	t = Topic("mqtt:///foo/bar")
	c.Assert(t.Valid(), Equals, true)
	c.Assert(t.Protocol(), Equals, "mqtt")
	c.Assert(t.Path(), Equals, "/foo/bar")

	t = Topic("mqtt://foo/bar")
	c.Assert(t.Valid(), Equals, false)
	t = Topic("mqt://foo/bar")
	c.Assert(t.Valid(), Equals, false)
	t = Topic("mqt://")
	c.Assert(t.Valid(), Equals, false)

	t = Topic("mqtt:///foo/bar")
	c.Assert(t.Path(), Equals, "/foo/bar")

}

func (suite *PubSubTests) TestBroker(c *C) {

	var b Broker
	b = Broker("mqtt://localhost:1281/foo")
	c.Assert(b.Valid(), Equals, false)

	b = Broker("http://localhost:1281")
	c.Assert(b.Valid(), Equals, false)

	b = Broker("mqtt://localhost:1281")
	c.Assert(b.Valid(), Equals, true)

	t := b.Topic("/this/is/topic")
	c.Assert(t.Path(), Equals, "/this/is/topic")
}
