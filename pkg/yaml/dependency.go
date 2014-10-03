package yaml

import (
	"errors"
	"fmt"
	"log"
)

type task struct {
	order            int
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

func (t *task) Run(c Context) {
	var mode = "LIVE "
	if c.test_mode() {
		mode = "TEST "
	}

	logf := "%6s %7s %4d %s\n"

	t.executions += 1
	if t.executions > 1 {
		log.Printf(logf, mode, "SKIP", t.order, t.description)
		return // already run
	}

	dispatched := 0
	for _, depend := range t.depends {
		go func(d *task) {
			d.Run(c) // this will send response via channels
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
		log.Printf(logf, mode, "ABORT", t.order, t.description)
		return
	}

	if kill {
		for _, up := range t.upstream {
			up.chan_errors <- errors.New(fmt.Sprintf("task-killed: %d %s", t.order, t.description))
		}
		return
	}

	var err error

	log.Printf(logf, mode, "PREPARE", t.order, t.description)
	err = t.self.Prepare(c)
	if err != nil {
		log.Printf(logf, "ERROR", "PREPARE", t.order, t.description)
		for _, up := range t.upstream {
			up.chan_errors <- err // propagate upward
		}
		return
	}

	log.Printf(logf, mode, "EXECUTE", t.order, t.description)
	err = t.self.Execute(c)
	if err != nil {
		log.Printf(logf, "ERROR", "EXECUTE", t.order, t.description)
		for _, up := range t.upstream {
			up.chan_errors <- err // propagate upward
		}
		return
	}

	log.Printf(logf, mode, "FINISH", t.order, t.description)
	err = t.self.Finish(c)
	if err != nil {
		log.Printf(logf, "ERROR", "FINISH", t.order, t.description)
		for _, up := range t.upstream {
			up.chan_errors <- err // propagate upward
		}
		return
	}

	// completion
	for _, up := range t.upstream {
		up.chan_completions <- 1 // propagate upward
	}
	return
}
