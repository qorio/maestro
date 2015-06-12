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

func (this *Task) Init(zk zk.ZK, pub pubsub.Publisher) (*task, error) {
	if !this.Success.Valid() {
		return nil, ErrBadConfig
	}
	if !this.Error.Valid() {
		return nil, ErrBadConfig
	}
	task := task{
		Task: *this,
		zk:   zk,
		pub:  pub,
	}

	task.status = make(chan []byte)

	if task.Task.Stdout != nil {
		task.stdout = make(chan []byte)
	}

	if task.Task.Stderr != nil {
		task.stderr = make(chan []byte)
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
		this.Log("Sending stdout to", this.Task.Stdout.String())
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
		this.Log("Sending stderr to", this.Task.Stderr.String())
	}
	return this.stdout, this.stderr, nil
}

func (this *task) Success(output interface{}) error {
	if this.done {
		return ErrStopped
	}
	if this.Task.Output != nil {
		value, err := json.Marshal(output)
		if err != nil {
			return err
		}
		err = zk.CreateOrSet(this.zk, *this.Task.Output, string(value))
		if err != nil {
			return err
		}
		this.Log("Success", "Result written to", this.Task.Output.String())
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
	this.Log("Error", "Error written to", this.Task.Error.String())
	this.Stop()
	return nil
}
