package task

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
	topic          = pubsub.Topic("mqtt://iot.eclipse.org:1883/test.com/task/test")
	topicIn        = pubsub.Topic("mqtt://iot.eclipse.org:1883/test.com/input")
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

func (suite *TaskTests) TestExecOnly(c *C) {
	t := Task{
		Id: "test",
		Cmd: &Cmd{
			Path: "echo",
			Args: []string{"hello"},
		},
		ExecOnly: true,
	}

	runtime, err := t.Init(nil)
	c.Assert(err, Equals, nil)
	runtime.CaptureStdout()

	done, err := runtime.Start()
	c.Assert(err, Equals, nil)
	c.Assert(done, Not(Equals), nil)
}

func (suite *TaskTests) TestTaskSuccess(c *C) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	info := registry.Path("/unit-test/task-test/task/test/" + now)
	stdout := &topic
	stderr := &topic
	status := &topic
	success := registry.Path("/unit-test/task-test/task/test/" + now + "/done")
	error := registry.Path("/unit-test/task-test/task/test/" + now + "/error")

	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Namespace: &info,
		Success:   &success,
		Error:     &error,
		Stdout:    stdout,
		Stderr:    stderr,
		LogTopic:  *status,
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
			c.Log("!!!!!message=", string(m))
			*count++
		}
	}()

	// now do work
	ch_stdout, ch_stderr, err := task.start_streams()

	c.Assert(err, Equals, nil)
	ch_stdout <- []byte("message to stdout")
	ch_stderr <- []byte("message to stderr")

	task.Log("this is a log message")

	err = task.Success(task.Task)
	c.Assert(err, Equals, nil)
	c.Assert(task.Running(), Equals, true)

	// We expect the success path to exist
	s := zk.GetString(z, success)
	c.Assert(s, Not(Equals), nil)

	c.Log("Count=", *count)
}

func (suite *TaskTests) TestTaskError(c *C) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	info := registry.Path("/unit-test/task-test/task/test2/" + now)
	stdout := &topic
	stderr := &topic
	status := &topic
	success := registry.Path("/unit-test/task-test/task/test2/" + now + "/done")
	error := registry.Path("/unit-test/task-test/task/test2/" + now + "/error")

	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Namespace: &info,
		Success:   &success,
		Error:     &error,
		Stdout:    stdout,
		Stderr:    stderr,
		LogTopic:  *status,
	}).Init(z)
	c.Assert(err, Equals, nil)
	c.Log("task=", task)

	// start a subscriber
	sub, err := suite.mq.Subscribe(topic)
	c.Assert(err, Equals, nil)
	go func() {
		for {
			m := <-sub
			c.Log(string(m))
		}
	}()

	// now do work
	ch_stdout, ch_stderr, err := task.start_streams()

	c.Assert(err, Equals, nil)
	ch_stdout <- []byte("message to stdout")
	ch_stderr <- []byte("message to stderr")

	task.Log("this is a log message")

	err = task.Error(task.Task)
	c.Assert(err, Equals, nil)
	c.Assert(task.Running(), Equals, true)

	task.Stop()
	c.Assert(task.Running(), Equals, false)

	s := zk.GetString(z, success)
	c.Assert(s, Equals, (*string)(nil))

	e := zk.GetString(z, error)
	c.Assert(e, Not(Equals), nil)
	c.Log("e object=", *e)
}

func (suite *TaskTests) TestTaskExec(c *C) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	info := registry.Path("/unit-test/task-test/task/testExec/" + now)
	stdout := &topic
	status := &topic
	success := registry.Path("/unit-test/task-test/task/testExec/" + now + "/done")
	error := registry.Path("/unit-test/task-test/task/testExec/" + now + "/error")

	exec := Cmd{
		Path: "ls",
		Args: []string{"-al"},
		Env:  []string{"FOO=foo"},
	}
	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Namespace: &info,
		Success:   &success,
		Error:     &error,
		Stdout:    stdout,
		LogTopic:  *status,
		Cmd:       &exec,
	}).Init(z)
	c.Assert(err, Equals, nil)
	c.Log("task=", task)

	task.CaptureStdout()

	done, err := task.Start()

	c.Assert(err, Equals, nil)

	result := <-done
	c.Assert(result, Equals, nil)

	buff := task.GetCapturedStdout()
	c.Log("Output=", string(buff))

}

func (suite *TaskTests) TestTaskCmdStdin(c *C) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	info := registry.Path("/unit-test/task-test/task/testCmd/" + now)
	stdin := &topicIn
	stdout := &topic
	status := &topic
	success := registry.Path("/unit-test/task-test/task/testCmd/" + now + "/done")
	error := registry.Path("/unit-test/task-test/task/testCmd/" + now + "/error")

	exec := Cmd{
		Path: "/bin/bash",
		Args: []string{},
		Env:  []string{"FOO=foo"},
	}
	z, err := zk.Connect([]string{"localhost:2181"}, 5*time.Second)
	c.Assert(err, Equals, nil)

	task, err := (&Task{
		Namespace: &info,
		Success:   &success,
		Error:     &error,
		Stdout:    stdout,
		Stdin:     stdin,
		LogTopic:  *status,
		Cmd:       &exec,
	}).Init(z)
	c.Assert(err, Equals, nil)
	c.Log("task=", task)

	task.CaptureStdout()

	done, err := task.Start()

	c.Assert(err, Equals, nil)

	// This will block while we wait for stdin
	pub, err := topicIn.Broker().PubSub("")
	c.Assert(err, Equals, nil)
	pw := pubsub.GetWriter(topicIn, pub)

	pw.Write([]byte("date\n"))
	pw.Write([]byte("ls -al\n"))
	pw.Write([]byte("pwd\n"))
	pw.Write([]byte("#bye"))

	c.Log("Should complete here'")
	result := <-done
	c.Assert(result, Equals, nil)

	buff := task.GetCapturedStdout()
	c.Log("Output=", string(buff))

}
