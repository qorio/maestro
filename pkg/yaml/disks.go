package yaml

import (
	"fmt"
)

func (this *Disk) Validate(c Context) error {
	return this.Size.Validate()
}

func (this *Disk) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Disk) Prepare(c Context) error {
	return nil
}

func (this *Disk) Execute(c Context) error {
	return nil
}

func (this *Disk) Finish(c Context) error {
	return nil
}

func (this *Disk) get_task() *task {
	if this.task == nil {
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Disk[%s]", this.name)
	}
	return this.task
}
