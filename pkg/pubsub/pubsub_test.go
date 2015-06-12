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
	c.Assert(t.Protocol("mqtt"), Equals, true)
	c.Assert(t.String(), Equals, "/foo/bar")

	t = Topic("mqtt://foo/bar")
	c.Assert(t.Protocol("mqtt"), Equals, true)
	c.Assert(t.String(), Equals, "foo/bar")

	t = Topic("mqtt://foo/bar")
	c.Assert(t.Protocol("http"), Equals, false)
	c.Assert(t.String(), Equals, "foo/bar")

}
