package yaml

import (
	"errors"
)

type task struct {
	depends          []task
	self             Runnable
	chan_errors      chan error
	chan_completions chan int
	task_errors      []error
}

func (task *task) Run(c Context) error {
	for _, depend := range task.depends {
		go func() {
			err := depend.Run(c)
			if err != nil {
				task.chan_errors <- err
			} else {
				task.chan_completions <- 1
			}
		}()
	}

	// wait here
	total := len(task.depends)
	select {
	case err := <-task.chan_errors:
		total -= 1
		task.task_errors = append(task.task_errors, err)
		if total == 0 {
			break
		}
	case <-task.chan_completions:
		total -= 1
		if total == 0 {
			break
		}
	}

	if len(task.task_errors) > 0 {
		return errors.New("task-failed")
	}

	var err error
	err = task.self.Prepare(c)
	if err != nil {
		return err
	}
	err = task.self.Execute(c)
	if err != nil {
		return err
	}
	err = task.self.Finish(c)
	if err != nil {
		return err
	}

	return nil
}
