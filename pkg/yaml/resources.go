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

func checkFile(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return errors.New(fmt.Sprint("File missing:", p, "err=", err))
	}
	if fi.IsDir() {
		return errors.New(fmt.Sprint("Is a dir:", p))
	}
	return nil
}

func (this *Instance) Validate(c Context) error {
	c.eval(&this.Cloud)
	c.eval(&this.Project)
	c.eval(&this.Keypair)
	// private key
	privateKey := this.Keypair
	if err := checkFile(privateKey); err != nil {
		return err
	}

	publicKey := this.Keypair + ".pub"
	if err := checkFile(publicKey); err != nil {
		return err
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
