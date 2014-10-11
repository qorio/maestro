package yaml

import (
	"fmt"
)

func (this *Container) Name() ContainerKey {
	return this.name
}

func (this *Container) Validate(c Context) error {
	return nil
}

func (this *Container) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Container) Prepare(c Context) error {
	return nil
}

func (this *Container) Execute(c Context) error {
	c.log("Executing Container %s", this.name)
	return nil
}

func (this *Container) Finish(c Context) error {
	return nil
}

func (this *Container) get_task() *task {
	if this.task == nil {
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Container[%s] @ Instance[%s]", this.name, this.targetInstance.name)

		if this.targetInstance != nil {
			running_instance := this.targetInstance.get_task()
			this.task.DependsOn(running_instance)
		}

		if this.targetImage != nil {
			container_image := this.targetImage.get_task()
			this.task.DependsOn(container_image)
		}
	}

	return this.task
}
