package task

import (
	. "github.com/qorio/maestro/pkg/registry"
	. "gopkg.in/check.v1"
	"testing"
)

func TestExec(t *testing.T) { TestingT(t) }

type ExecTests struct{}

var _ = Suite(&ExecTests{})

func (suite *ExecTests) TestApplySubs(c *C) {
	c1 := Cmd{
		Dir:  ".",
		Path: "/usr/bin/ls",
		Args: []string{"-l", "foo.*"},
		Env:  []string{"FOO=BAR", "BAR=BAZ"},
	}

	c2 := c1

	c.Assert(len(c2.Args), Equals, len(c2.Args))
	c.Assert(len(c2.Env), Equals, len(c2.Env))
	c.Assert(c2.Args, DeepEquals, c1.Args)
	c.Assert(c2.Env, DeepEquals, c1.Env)
}
