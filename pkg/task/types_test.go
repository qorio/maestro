package task

import (
	"encoding/json"
	. "github.com/qorio/maestro/pkg/registry"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func TestTypes(t *testing.T) { TestingT(t) }

type TypesTests struct{}

var _ = Suite(&TypesTests{})

func (suite *TypesTests) TestUnmarshalMarshal(c *C) {

	input := `{
		"info" : "/{{.Domain}}/deployment/{{.Id}}/db-migrate",
		"trigger" : {
                    "registry": {
		        "timeout" : "300s",
                        "members" : {
            		    "path" : "/{{.Domain}}/passport-db-master/containers",
		            "equals" : 1
                        }
                    }
		},
		"scheduler" : "passport-db-migrate",
		"workers" : "exclusive",
		"success" : "/{{.Domain}}/deployment/{{.Id}}/db-seed",
		"error" : "/{{.Domain}}/deployment/{{.Id}}/exception"
	    }`

	t := new(Task)
	err := json.Unmarshal([]byte(input), t)
	c.Assert(err, Equals, nil)

	c.Assert(t.Info, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/db-migrate"))
	c.Assert(t.Trigger.Registry.Members.Top, Equals, Path("/{{.Domain}}/passport-db-master/containers"))
	c.Assert(time.Duration(*t.Trigger.Registry.Timeout).Seconds(), Equals, float64(300))
	c.Assert(t.Success, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/db-seed"))
	c.Assert(t.Error, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/exception"))

	// marshal
	m, err := json.Marshal(t)
	c.Assert(err, Equals, nil)

	// and back
	t = new(Task)
	err = json.Unmarshal(m, t)
	c.Assert(err, Equals, nil)

	c.Assert(t.Info, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/db-migrate"))
	c.Assert(t.Trigger.Registry.Members.Top, Equals, Path("/{{.Domain}}/passport-db-master/containers"))
	c.Assert(time.Duration(*t.Trigger.Registry.Timeout).Seconds(), Equals, float64(300))
	c.Assert(t.Success, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/db-seed"))
	c.Assert(t.Error, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/exception"))

	c.Log(string(m))

}
