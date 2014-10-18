package yaml

import (
	"errors"
	"fmt"
)

func (this *Container) Name() ContainerKey {
	return this.name
}

func (this *Container) Validate(c Context) error {
	if this.targetInstance == nil {
		return errors.New("no-target-instance")
	}
	if this.targetInstance.ExternalIp == "" || this.targetInstance.InternalIp == "" {
		return errors.New("no-host-ip")
	}

	if this.targetInstance.Keypair == "" {
		return errors.New("no-keypair-for-auth")
	}
	return nil
}

func (this *Container) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Container) Prepare(c Context) error {
	return nil
}

func (this *Container) Execute(c Context) error {
	// SSH
	if this.Ssh != nil {
		if _, has := ip_map[this.targetInstance]; !has {
			err := this.Validate(c)
			if err != nil {
				return err
			}
		}

		client, err := ssh_client(ip_map[this.targetInstance], this.targetInstance.Keypair)
		if err != nil {
			return err
		}
		for i, cmd := range this.Ssh {
			c.log("   Container[%12s] -- ssh[%2d]: %s", this.name, i, *cmd)
			if !c.test_mode() {
				stdout, err := client.RunCommandStdout(*cmd)
				if err != nil {
					c.log("ERROR - %s", err.Error())
					return err
				} else {
					c.log("   Container[%12s] -- stdout[%2d]: %s", string(stdout))
				}
			}
		}
	}
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
