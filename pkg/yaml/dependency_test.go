package yaml

import (
	"errors"
	. "gopkg.in/check.v1"
	"sort"
	"testing"
)

func TestYamlDependencies(t *testing.T) { TestingT(t) }

type YamlDependenciesTests struct{}

var _ = Suite(&YamlDependenciesTests{})

var queue = []int{}

type t1 int

func (t t1) Prepare(c Context) error {
	return nil
}
func (t t1) Execute(c Context) error {
	queue = append(queue, int(t))
	return nil
}
func (t t1) Finish(c Context) error {
	return nil
}

type t2 int

func (t t2) Prepare(c Context) error {
	return nil
}
func (t t2) Execute(c Context) error {
	queue = append(queue, int(t))
	return errors.New("t2-error")
}
func (t t2) Finish(c Context) error {
	return nil
}

type ct chan bool

func (t ct) Prepare(c Context) error {
	return nil
}
func (t ct) Execute(c Context) error {
	<-t // just blocks
	return nil
}
func (t ct) Finish(c Context) error {
	return nil
}

func (suite *YamlDependenciesTests) TestRun(c *C) {
	queue = []int{}

	task0 := alloc_task(t1(0))
	task0.order = 0
	task0.description = "task0"

	task10 := alloc_task(t1(10))
	task10.order = 10
	task10.description = "task10"

	task11 := alloc_task(t1(11))
	task11.order = 11
	task11.description = "task11"

	task20 := alloc_task(t1(20))
	task20.order = 20
	task20.description = "task20"

	task11.DependsOn(task20)
	task10.DependsOn(task11)
	task0.DependsOn(task10)

	context := make(Context)
	context[LIVE_MODE] = "true"

	c.Assert(context.test_mode(), Equals, false)

	task0.Run(context)
	c.Assert(len(task0.task_errors), Equals, 0)

	c.Log("queue=", queue)

	for i, v := range []int{20, 11, 10, 0} {
		c.Assert(v, Equals, queue[i])
	}

}

func (suite *YamlDependenciesTests) TestRunParallel(c *C) {
	queue = []int{}

	task0 := alloc_task(t1(0))
	task0.order = 0
	task0.description = "task0"

	task10 := alloc_task(t1(10))
	task10.order = 10
	task10.description = "task10"

	task11 := alloc_task(t1(11))
	task11.order = 11
	task11.description = "task11"

	task20 := alloc_task(t1(20))
	task20.order = 20
	task20.description = "task20"

	// A diamond
	task10.DependsOn(task20)
	task11.DependsOn(task20)
	task0.DependsOn(task10, task11)

	context := make(Context)
	context[LIVE_MODE] = "false"

	c.Assert(context.test_mode(), Equals, true)

	task0.Run(context)
	c.Assert(len(task0.task_errors), Equals, 0)

	c.Log("queue=", queue)

	c.Assert(queue[len(queue)-1], Equals, 0)
	c.Assert(queue[0], Equals, 20)

	sort.IntSlice(queue[1:2]).Sort()
	c.Assert(queue[1], Equals, 10)
	c.Assert(queue[2], Equals, 11)
}

func (suite *YamlDependenciesTests) TestRunParallelWithError(c *C) {
	queue = []int{}

	task0 := alloc_task(t1(0))
	task0.order = 0
	task0.description = "task0"

	task10 := alloc_task(t1(10))
	task10.order = 10
	task10.description = "task10"

	task11 := alloc_task(t1(11))
	task11.order = 11
	task11.description = "task11"

	task20 := alloc_task(t2(20))
	task20.order = 20
	task20.description = "task20"

	// A diamond
	task10.DependsOn(task20)
	task11.DependsOn(task20)
	task0.DependsOn(task10, task11)

	context := make(Context)
	context[LIVE_MODE] = "false"

	c.Assert(context.test_mode(), Equals, true)

	task0.Run(context)
	c.Assert(len(task0.task_errors), Not(Equals), 0)

	c.Log("errors=", task0.task_errors)
	c.Log("queue=", queue)

	// We should have executed only 1 which failed.
	c.Assert(queue[len(queue)-1], Equals, 20)
	c.Assert(len(queue), Equals, 1)
}

func (suite *YamlDependenciesTests) TestRunParallelWithErrorInParllelTask(c *C) {

	queue = []int{}

	task0 := alloc_task(t1(0))
	task0.order = 0
	task0.description = "task0"

	task10 := alloc_task(t2(10)) // will fail here
	task10.order = 10
	task10.description = "task10"

	task11 := alloc_task(t1(11))
	task11.order = 11
	task11.description = "task11"

	task20 := alloc_task(t1(20))
	task20.order = 20
	task20.description = "task20"

	// A diamond
	task10.DependsOn(task20)
	task11.DependsOn(task20)
	task0.DependsOn(task10, task11)

	context := make(Context)
	context[LIVE_MODE] = "true"

	c.Assert(context.test_mode(), Equals, false)

	task0.Run(context)
	c.Assert(len(task0.task_errors), Not(Equals), 0)

	c.Log("errors=", task0.task_errors)
	c.Log("queue=", queue)

	expects := []int{20, 10, 11} // Note that 0 is not run because 10 failed.
	for i, exp := range expects {
		c.Assert(queue[i], Equals, exp)
	}
}
