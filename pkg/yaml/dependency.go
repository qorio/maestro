package yaml

import (
	"errors"
	"fmt"
)

type task struct {
	description      string
	depends          []*task
	upstream         []*task
	self             Runnable
	chan_errors      chan error
	chan_completions chan int
	task_errors      []error
	chan_kill        chan bool
	executions       int
}

func alloc_task(r Runnable) *task {
	return &task{
		depends:          []*task{},
		upstream:         []*task{},
		chan_errors:      make(chan error),
		chan_completions: make(chan int),
		task_errors:      []error{},
		self:             r,
		chan_kill:        make(chan bool),
	}
}

func (t *task) DependsOn(tt ...*task) *task {
	if t.depends == nil {
		t.depends = []*task{}
	}
	for _, k := range tt {
		t.depends = append(t.depends, k)
		k.upstream = append(k.upstream, t)
	}
	return t
}

func (t *task) Kill() {
	t.chan_kill <- true
}

func (t *task) Reset() {
	t.executions = 0
	t.task_errors = []error{}
}

func (t *task) Run(c Context) error {
	logf := "%7s %s\n"
	error_logf := "%7s %s error=%s"

	t.executions += 1
	if t.executions > 1 {
		c.log(logf, "SKIP", t.description)
		return nil // already run
	}

	var err error

	c.log(logf, "PREPARE", t.description)
	err = t.self.Prepare(c)
	if err != nil {
		c.error(error_logf, "PREPARE", t.description, err.Error())
		for _, up := range t.upstream {
			up.chan_errors <- err // propagate upward
		}
		return err
	}

	dispatched := 0
	for _, depend := range t.depends {
		go func(d *task) {
			// make a copy of context
			cc := make(Context)
			cc.copy_from(c)
			d.Run(cc) // this will send response via channels
		}(depend)
		dispatched += 1
	}

	// wait here
	kill := false
	var sub_err error
	for dispatched > 0 {
		select {
		case err := <-t.chan_errors:
			dispatched -= 1
			t.task_errors = append(t.task_errors, err)
			sub_err = err
		case <-t.chan_completions:
			dispatched -= 1
		case kill = <-t.chan_kill:
		}

		if sub_err != nil {
			break
		}
	}

	if sub_err != nil {
		for _, up := range t.upstream {
			up.chan_errors <- sub_err // propagate upward
		}
		c.error(error_logf, "ABORT", t.description, sub_err.Error())
		return sub_err
	}

	if kill {
		for _, up := range t.upstream {
			up.chan_errors <- errors.New(fmt.Sprintf("task-killed: %d %s", t.description))
		}
		return errors.New("killed")
	}

	c.log(logf, "EXECUTE", t.description)
	err = t.self.Execute(c)
	if err != nil {
		c.error(error_logf, "EXECUTE", t.description, err.Error())
		for _, up := range t.upstream {
			up.chan_errors <- err // propagate upward
		}
		return err
	}

	c.log(logf, "FINISH", t.description)
	err = t.self.Finish(c)
	if err != nil {
		c.error(error_logf, "FINISH", t.description, err.Error())
		for _, up := range t.upstream {
			up.chan_errors <- err // propagate upward
		}
		return err
	}

	// completion
	for _, up := range t.upstream {
		up.chan_completions <- 1 // propagate upward
	}
	return nil
}
