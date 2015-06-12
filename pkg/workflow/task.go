package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/zk"
	"runtime"
	"strings"
	"time"
)

var (
	ErrBadConfig = errors.New("bad-config")
	ErrStopped   = errors.New("stopped")
)

type task struct {
	Task

	zk  zk.ZK
	pub pubsub.Publisher

	status chan []byte
	stdout chan []byte
	stderr chan []byte

	done bool
}

func (this *Task) Validate() error {
	switch {
	case !this.Info.Valid():
		return ErrBadConfig
	case !this.Success.Valid():
		return ErrBadConfig
	case !this.Error.Valid():
		return ErrBadConfig
	}
	return nil
}

func (this *Task) Init(zkc zk.ZK, pub pubsub.Publisher) (*task, error) {
	if err := this.Validate(); err != nil {
		return nil, err
	}

	task := task{
		Task: *this,
		zk:   zkc,
		pub:  pub,
	}

	task.status = make(chan []byte)

	if task.Task.Stdout != nil {
		task.stdout = make(chan []byte)
	}

	if task.Task.Stderr != nil {
		task.stderr = make(chan []byte)
	}

	now := time.Now()
	task.Stat.Started = &now
	err := zk.CreateOrSet(task.zk, task.Info, task.Stat)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (this *task) Stop() {
	if this.done {
		return
	}

	if this.stdout != nil {
		this.stdout <- nil
	}
	if this.stderr != nil {
		this.stderr <- nil
	}
	this.Log("Stop")
	this.status <- nil

	this.done = true
}

func (this *task) Log(m ...string) {
	if this.done {
		return
	}
	source := ""
	_, file, line, ok := runtime.Caller(1)
	if ok {
		source = fmt.Sprintf("%s:%d", file, line)
	}

	s := strings.Join(m, " ")
	this.status <- []byte(s)
	glog.Infoln(source, m)
}

func (this *task) Running() bool {
	return !this.done
}

func (this *task) Start() (stdout, stderr chan<- []byte, err error) {
	if this.done {
		return nil, nil, ErrStopped
	}
	go func() {
		for {
			m := <-this.status
			if m == nil {
				break
			}
			this.pub.Publish(this.Task.Status, m)
		}
	}()
	if this.stdout != nil {
		go func() {
			for {
				m := <-this.stdout
				if m == nil {
					break
				}
				this.pub.Publish(*this.Task.Stdout, m)
			}
		}()
		this.Log("Sending stdout to", this.Task.Stdout.Path())
	}
	if this.stderr != nil {
		go func() {
			for {
				m := <-this.stderr
				if m == nil {
					break
				}
				this.pub.Publish(*this.Task.Stderr, m)
			}
		}()
		this.Log("Sending stderr to", this.Task.Stderr.Path())
	}
	return this.stdout, this.stderr, nil
}

func (this *task) Success(output interface{}) error {
	if this.done {
		return ErrStopped
	}

	value, err := json.Marshal(output)
	if err != nil {
		return err
	}
	err = zk.CreateOrSetBytes(this.zk, this.Task.Success, value)
	if err != nil {
		return err
	}

	// copy the data over
	if this.Task.Output != nil {
		err = zk.CreateOrSetBytes(this.zk, *this.Task.Output, value)
		if err != nil {
			return err
		}
		this.Log("Success", "Result written to", this.Task.Output.Path())
	}

	now := time.Now()
	this.Stat.Success = &now
	err = zk.CreateOrSet(this.zk, this.Info, this.Stat)
	if err != nil {
		return err
	}

	this.Log("Success", "Completed")
	this.Stop()
	return nil
}

func (this *task) Error(error interface{}) error {
	if this.done {
		return ErrStopped
	}
	value, err := json.Marshal(error)
	if err != nil {
		return err
	}
	err = zk.CreateOrSet(this.zk, this.Task.Error, string(value))
	if err != nil {
		return err
	}

	now := time.Now()
	this.Stat.Error = &now
	err = zk.CreateOrSet(this.zk, this.Info, this.Stat)
	if err != nil {
		return err
	}

	this.Log("Error", "Error written to", this.Task.Error.Path())
	this.Stop()
	return nil
}
