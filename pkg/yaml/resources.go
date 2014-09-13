package yaml

import (
	"errors"
	"fmt"
	"os"
)

func (this *Disk) Validate(c Context) error {
	return nil
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

func (this *Instance) Validate(c Context) error {
	c.eval(&this.Cloud)
	c.eval(&this.Project)
	c.eval(&this.Keyfile)
	fi, err := os.Stat(this.Keyfile)
	if err != nil {
		return errors.New(fmt.Sprint("Missing keyfile at", this.Keyfile, ":", err))
	}
	if fi.IsDir() {
		return errors.New(fmt.Sprint("Keyfile", this.Keyfile, "is a directory."))
	}

	return nil
}

func (this *Instance) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Instance) Prepare(c Context) error {
	return nil
}

func (this *Instance) Execute(c Context) error {
	return nil
}

func (this *Instance) Finish(c Context) error {
	return nil
}
