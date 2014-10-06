package yaml

import (
	. "gopkg.in/check.v1"
	"testing"
)

func TestParse(t *testing.T) { TestingT(t) }

type ParseTests struct{}

var _ = Suite(&ParseTests{})

func (suite *ParseTests) TestParseSizeQuantityUnit(c *C) {
	var v SizeQuantityUnit

	v = SizeQuantityUnit("20g")
	c.Assert(v.ToUnit(KB), Equals, 20*1000000.)
	c.Assert(v.ToUnit(GB), Equals, 20*1.)
	c.Assert(v.ToUnit(MB), Equals, 20*1000.)
	c.Assert(v.ToUnit(TB), Equals, 0.02)
	c.Log(v, " = ", v.ToUnit(KB), " KB")
	c.Log(v, " = ", v.ToUnit(GB), " GB")
	c.Log(v, " = ", v.ToUnit(MB), " MB")
	c.Log(v, " = ", v.ToUnit(TB), " TB")

	v = SizeQuantityUnit("1M")
	c.Assert(v.ToUnit(MB), Equals, 1.)
	c.Assert(v.ToUnit(KB), Equals, 1000.)
	c.Assert(v.ToUnit(GB), Equals, 0.001)
	c.Assert(v.ToUnit(TB), Equals, 0.000001)
	c.Log(v, " = ", v.ToUnit(KB), " KB")
	c.Log(v, " = ", v.ToUnit(GB), " GB")
	c.Log(v, " = ", v.ToUnit(MB), " MB")
	c.Log(v, " = ", v.ToUnit(TB), " TB")

}
