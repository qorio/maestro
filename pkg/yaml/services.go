package yaml

import (
	"errors"
	"fmt"
)

func (this *Service) Name() ServiceKey {
	return this.name
}

func (this *Service) Targets() [][]*Container {
	// all the container instances for all the jobs
	list := [][]*Container{}
	for _, job := range this.jobs {
		clist := []*Container{}
		for _, c := range job.container_instances {
			clist = append(clist, c)
		}
		list = append(list, clist)
	}
	return list
}

func (this *Service) Validate(c Context) error {
	// Do variable substitutions
	for _, group := range this.Targets() {
		for _, container := range group {
			if container.targetInstance == nil {
				return errors.New(fmt.Sprint("No instance assigned for container", container.name))
			}

			cc := make(Context)
			cc.copy_from(c)
			cc.eval(&container.ImageRef)

			if container.targetImage == nil && container.ImageRef == "" {
				return errors.New(fmt.Sprint("No image for container", container.name))
			}
			if container.targetImage == nil {
				cc["image"] = map[string]interface{}{
					"id": container.ImageRef,
				}
			} else {
				cc["image"] = container.targetImage.export_vars()
			}
			cc["instance"] = container.targetInstance.export_vars()
			for i, ssh := range container.Ssh {
				old := cc.eval(ssh)
				if *container.Ssh[i] == "" {
					return errors.New(fmt.Sprint("Failed to evaluate '", old, "' for container", container.name))
				}
			}
		}
	}

	return nil
}

func (this *Service) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Service) Prepare(c Context) error {
	return nil
}

func (this *Service) Execute(c Context) error {
	return nil
}

func (this *Service) Finish(c Context) error {
	return nil
}

func (this *Service) get_task() *task {
	if this.task == nil {
		// Service depends on the jobs starting up in sequence
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Service[%s]", string(this.name))

		t := this.task
		for i := len(this.jobs) - 1; i >= 0; i-- {
			j := this.jobs[i].get_task()
			t.DependsOn(j)
			t = j
		}
	}
	return this.task
}
