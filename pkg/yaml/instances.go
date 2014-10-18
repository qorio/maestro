package yaml

import (
	"errors"
	"fmt"
	"github.com/qorio/maestro/pkg/ssh"
)

var ip_map map[*Instance]Ip

func init() {
	ip_map = make(map[*Instance]Ip, 0)
}

func ssh_client(host Ip, keypath string) (*ssh.Client, error) {
	// Get the login user from the public key
	pk, err := ssh.ParsePublicKeyFile(keypath + ".pub")
	if err != nil {
		return nil, err
	}

	auth, err := ssh.KeyFileAuthMethod(keypath)
	if err != nil {
		return nil, err
	}

	return ssh.NewClient(pk.User, string(host), auth)
}

func (this *Instance) export_vars() map[string]interface{} {
	vm := make(map[string]map[string]interface{})
	for k, volume := range this.disks {
		vm[string(k)] = map[string]interface{}{
			"mount": volume.MountPoint,
			"disk":  volume.Disk,
		}
	}
	return map[string]interface{}{
		"volumes": vm,
		"name":    this.name,
	}
}

func (this *Instance) Validate(c Context) error {
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
	// Do a quick connection
	c.log("   Instance[%12s] -- checking ssh connection to %s", this.name, this.ExternalIp)
	_, err := ssh_client(this.ExternalIp, this.Keypair)
	if err != nil {
		c.log("   Instance[%12s] -- checking ssh connection to %s", this.name, this.InternalIp)
		_, err2 := ssh_client(this.InternalIp, this.Keypair)

		if err2 != nil {
			return errors.New("cannot-connect-to-ip")
		} else {
			ip_map[this] = this.InternalIp
		}
	} else {
		ip_map[this] = this.ExternalIp
	}
	return nil
}

func (this *Instance) Execute(c Context) error {
	c.log("Executing instance %s", this.name)

	// TODO - This launches the machine instance
	return nil
}

func (this *Instance) Finish(c Context) error {
	return nil
}

func (this *Instance) can_take(job *Job) bool {
	return intersect(this.labels, job.instance_labels)
}

// For sorting by instance key
type ByInstanceKey []*Instance

func (a ByInstanceKey) Len() int {
	return len(a)
}
func (a ByInstanceKey) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a ByInstanceKey) Less(i, j int) bool {
	return a[i].name < a[j].name
}

func (this *Instance) get_task() *task {
	if this.task == nil {
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Instance[%s]", this.name)
		// TODO - add ip
		for _, volume := range this.disks {
			d := volume.disk.get_task()
			this.task.DependsOn(d)
		}
	}
	return this.task
}
