package workflow

import (
	"fmt"
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
	topic          = pubsub.Topic("mqtt://iot.eclipse.org:1883/this/is/a/test")
)

func TestTask(t *testing.T) { TestingT(t) }

type TaskTests struct {
	mq pubsub.PubSub
}

var _ = Suite(&TaskTests{})

func (suite *TaskTests) SetUpSuite(c *C) {
	c.Assert(topic.Valid(), Equals, true)
	c.Assert(topic.Protocol(), Equals, "mqtt")
	mq, err := mqtt.Connect("test", local_endpoint)
	c.Assert(err, Equals, nil)

	suite.mq = mq
}

func (suite *TaskTests) TestTaskSuccess(c *C) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	info := registry.Path("/ops-test/task/test/" + now)
	output := registry.Path("/ops-test/task/test/" + now + "/out")
	stdout := &topic
	stderr := &topic
	status := &topic
	success := registry.Path("/ops-test/task/test/" + now + "/done")
	error := registry.Path("/ops-test/task/test/" + now + "/error")

	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Info:    info,
		Success: success,
		Error:   error,
		Output:  &output,
		Stdout:  stdout,
		Stderr:  stderr,
		Status:  *status,
	}).Init(z)
	c.Assert(err, Equals, nil)
	c.Log("task=", task)

	// start a subscriber
	sub, err := suite.mq.Subscribe(topic)
	c.Assert(err, Equals, nil)

	count := new(int)
	*count = 0
	go func() {

		for {
			m := <-sub
			c.Log("message=", string(m))
			*count++
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
	v := zk.GetString(z, output)
	c.Assert(v, Not(Equals), nil)
	c.Log("v=", *v)

	// We expect the success path to exist
	s := zk.GetString(z, success)
	c.Assert(s, Not(Equals), nil)

	c.Assert(*count, Not(Equals), 0)
}

func (suite *TaskTests) TestTaskError(c *C) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	info := registry.Path("/ops-test/task/test2/" + now)
	output := registry.Path("/ops-test/task/test2/" + now + "/out")
	stdout := &topic
	stderr := &topic
	status := &topic
	success := registry.Path("/ops-test/task/test2/" + now + "/done")
	error := registry.Path("/ops-test/task/test2/" + now + "/error")

	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Info:    info,
		Success: success,
		Error:   error,
		Output:  &output,
		Stdout:  stdout,
		Stderr:  stderr,
		Status:  *status,
	}).Init(z)
	c.Assert(err, Equals, nil)
	c.Log("task=", task)

	// start a subscriber
	sub, err := suite.mq.Subscribe(topic)
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

	err = task.Error(task.Task)
	c.Assert(err, Equals, nil)
	c.Assert(task.Running(), Equals, false)

	// Check for the value at the output path
	v := zk.GetString(z, output)
	c.Assert(v, Equals, (*string)(nil))

	s := zk.GetString(z, success)
	c.Assert(s, Equals, (*string)(nil))

	e := zk.GetString(z, error)
	c.Assert(e, Not(Equals), nil)
	c.Log("e object=", *e)
}
