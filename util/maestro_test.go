package util

import (
	. "gopkg.in/check.v1"
	"runtime"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type suite struct{}

var _ = Suite(&suite{})

func (suite *suite) TestDummy1(c *C) {

	c.Assert(100, Equals, 100) //, "100 equals 100?")
	c.Assert(1, Equals, 2, Commentf("#CPUs == %d", runtime.NumCPU()))
}
