package workflow

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
		"start" : "/{{.Domain}}/deployment/{{.Id}}/db-migrate",
		"condition" : {
		    "path" : "/{{.Domain}}/passport-db-master/containers",
		    "min_children" : 1,
		    "timeout" : "300s"
		},
		"scheduler" : "passport-db-migrate",
		"workers" : "exclusive",
		"success" : "/{{.Domain}}/deployment/{{.Id}}/db-seed",
		"error" : "/{{.Domain}}/deployment/{{.Id}}/exception"
	    }`

	t := new(Task)
	err := json.Unmarshal([]byte(input), t)
	c.Assert(err, Equals, nil)

	c.Assert(*t.StartTrigger, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/db-migrate"))
	c.Assert(*t.WorkerPolicy, Equals, WorkerPolicy("exclusive"))
	c.Assert(t.Success, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/db-seed"))
	c.Assert(t.Error, Equals, Path("/{{.Domain}}/deployment/{{.Id}}/exception"))

	c.Assert(time.Duration(*t.Condition.Timeout).Seconds(), Equals, float64(300))

	// marshal
	m, err := json.Marshal(t)
	c.Assert(err, Equals, nil)

	c.Log(string(m))

}
