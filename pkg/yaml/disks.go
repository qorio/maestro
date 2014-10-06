package yaml

import ()

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
