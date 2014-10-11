package yaml

import (
	"fmt"
)

func (this *Job) Name() JobKey {
	return this.name
}

func (this *Job) Validate(c Context) error {
	return nil
}

func (this *Job) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Job) Prepare(c Context) error {
	return nil
}

func (this *Job) Execute(c Context) error {
	c.log("Executing Job %s", this.name)

	// TODO - build the port mapping / load balancer
	return nil
}

func (this *Job) Finish(c Context) error {
	return nil
}

func (this *Job) get_task() *task {
	if this.task == nil {
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Job[%s]", this.name)

		// ultimately container depends on an instance being ready, but it will just wait until the instances are ready
		for _, container := range this.container_instances {
			this.task.DependsOn(container.get_task())
		}
	}
	return this.task
}
