package task

import (
	. "github.com/qorio/maestro/pkg/registry"
	. "gopkg.in/check.v1"
	"testing"
)

func TestExec(t *testing.T) { TestingT(t) }

type ExecTests struct{}

var _ = Suite(&ExecTests{})

func (suite *ExecTests) TestDeepCopy(c *C) {
	c1 := Cmd{
		Dir:  "{{.WorkDir}}",
		Path: "prog",
		Args: []string{"-l", "-p {{.Port}}", "foo.*"},
		Env:  []string{"FOO=BAR", "BAR=BAZ", "DOMAIN={{.Domain}}"},
	}
	c2, err := c1.Copy()
	c.Assert(err, Equals, nil)
	c.Assert(c1, DeepEquals, *c2)
}

func (suite *ExecTests) TestApplySubs(c *C) {
	c1 := Cmd{
		Dir:  "{{.WorkDir}}",
		Path: "prog",
		Args: []string{"-l", "-p {{.Port}}", "foo.*"},
		Env:  []string{"FOO=BAR", "BAR=BAZ", "DOMAIN={{.Domain}}"},
	}

	c2, err := c1.ApplySubstitutions(map[string]interface{}{
		"WorkDir": "/tmp",
		"Port":    8080,
		"Domain":  "foo.com",
	}, nil)

	c.Assert(err, Equals, nil)
	c.Assert(len(c2.Args), Equals, len(c2.Args))
	c.Assert(len(c2.Env), Equals, len(c2.Env))

	c.Assert(c2.Dir, Equals, "/tmp")
	c.Assert(c2.Args, DeepEquals, []string{"-l", "-p 8080", "foo.*"})
	c.Assert(c2.Env, DeepEquals, []string{"FOO=BAR", "BAR=BAZ", "DOMAIN=foo.com"})

	c.Assert(c1.Args, DeepEquals, []string{"-l", "-p {{.Port}}", "foo.*"})
	c.Assert(c1.Env, DeepEquals, []string{"FOO=BAR", "BAR=BAZ", "DOMAIN={{.Domain}}"})
}
