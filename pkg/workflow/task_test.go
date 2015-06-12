package workflow

import (
	"github.com/qorio/maestro/pkg/mqtt"
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

var (
	local_endpoint = "iot.eclipse.org:1883"
	topic          = pubsub.Topic("mqtt:///this-is-a-test")
)

func TestTask(t *testing.T) { TestingT(t) }

type TaskTests struct{}

var _ = Suite(&TaskTests{})

func (suite *TaskTests) TestTask(c *C) {
	output := registry.Path("/ops-test/task/test/out")
	stdout := &topic
	stderr := &topic
	status := &topic
	success := registry.Path("/ops-test/task/test/done")
	error := registry.Path("/ops-test/task/test/error")

	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	mq, err := mqtt.Connect("test", local_endpoint)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Success: success,
		Error:   error,
		Output:  &output,
		Stdout:  stdout,
		Stderr:  stderr,
		Status:  *status,
	}).Init(z, mq)
	c.Assert(err, Equals, nil)
	c.Log("task=", task)

	// start a subscriber
	sub, err := mq.Subscribe(topic)
	c.Assert(err, Equals, nil)
	go func() {

		for {
			m := <-sub
			c.Log("message=", string(m))
		}
	}()

	// now do work
	ch_stdout, ch_stderr, err := task.Start()

	c.Assert(err, Equals, nil)
	ch_stdout <- []byte("message to stdout")
	ch_stderr <- []byte("message to stderr")

	task.Log("this is a log message")

	err = task.Success(task.Task)
	c.Assert(err, Equals, nil)
	c.Assert(task.Running(), Equals, false)

	// Check for the value at the output path
	v := zk.GetValue(z, output)
	c.Assert(v, Not(Equals), nil)
	c.Log("v=", *v)
}
