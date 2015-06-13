package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/zk"
	"io"
	"runtime"
	"strings"
	"time"
)

var (
	ErrBadConfig = errors.New("bad-config")
	ErrStopped   = errors.New("stopped")
)

type Runtime struct {
	Task

	zk zk.ZK

	status chan []byte
	stdout chan []byte
	stderr chan []byte
	stdin  chan []byte

	options interface{}
	done    bool
	ready   bool
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

func (this *Task) Init(zkc zk.ZK, options ...interface{}) (*Runtime, error) {
	if err := this.Validate(); err != nil {
		return nil, err
	}

	task := Runtime{
		Task: *this,
		zk:   zkc,
	}
	if len(options) > 0 {
		task.options = options[0]
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

func (this *Runtime) Stop() {
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

func (this *Runtime) Stdin() io.Reader {
	if this.Task.Stdin == nil {
		return nil
	}
	if c, err := this.Task.Stdin.Broker().PubSub(this.Id, this.options); err == nil {
		return pubsub.GetReader(*this.Task.Stdin, c)
	} else {
		return nil
	}
}

func (this *Runtime) Stdout() io.Writer {
	if this.Task.Stdout == nil {
		return nil
	}
	if c, err := this.Task.Stdout.Broker().PubSub(this.Id, this.options); err == nil {
		return pubsub.GetWriter(*this.Task.Stdout, c)
	} else {
		glog.Warningln("Error getting stdout.", "Topic=", *this.Task.Stdout, "Err=", err)
		return nil
	}
}

func (this *Runtime) Stderr() io.Writer {
	if this.Task.Stderr == nil {
		return nil
	}
	if c, err := this.Task.Stderr.Broker().PubSub(this.Id, this.options); err == nil {
		return pubsub.GetWriter(*this.Task.Stderr, c)
	} else {
		glog.Warningln("Error getting stderr.", "Topic=", *this.Task.Stderr, "Err=", err)
		return nil
	}
}

func (this *Runtime) Log(m ...string) {
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

func (this *Runtime) Running() bool {
	return !this.done
}

func (this *Runtime) Start() (stdout, stderr chan<- []byte, err error) {
	if this.ready {
		return this.stdout, this.stderr, nil
	}

	if this.done {
		return nil, nil, ErrStopped
	}
	go func() {
		for {
			m := <-this.status
			if m == nil {
				break
			}
			if c, err := this.Task.Status.Broker().PubSub(this.Id, this.options); err == nil {
				c.Publish(this.Task.Status, m)
			} else {
				glog.Warningln("Cannot publish:", this.Task.Status.String(), "Err=", err)
			}
		}
	}()
	if this.stdout != nil {
		go func() {
			for {
				m := <-this.stdout
				if m == nil {
					break
				}
				if c, err := this.Task.Stdout.Broker().PubSub(this.Id, this.options); err == nil {
					c.Publish(*this.Task.Stdout, m)
				} else {
					glog.Warningln("Cannot publish:", this.Task.Stdout.String(), "Err=", err)
				}

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
				if c, err := this.Task.Stderr.Broker().PubSub(this.Id, this.options); err == nil {
					c.Publish(*this.Task.Stderr, m)
				} else {
					glog.Warningln("Cannot publish:", this.Task.Stderr.String(), "Err=", err)
				}
			}
		}()
		this.Log("Sending stderr to", this.Task.Stderr.Path())
	}
	this.ready = true
	return this.stdout, this.stderr, nil
}

func (this *Runtime) Success(output interface{}) error {
	if this.done {
		return ErrStopped
	}

	switch output.(type) {
	case []byte:
		err := zk.CreateOrSetBytes(this.zk, this.Task.Success, output.([]byte))
		if err != nil {
			return err
		}
		// copy the data over
		if this.Task.Output != nil {
			err := zk.CreateOrSetBytes(this.zk, *this.Task.Output, output.([]byte))
			if err != nil {
				return err
			}
			this.Log("Success", "Result written to", this.Task.Output.Path())
		}
	case string:
		err := zk.CreateOrSetString(this.zk, this.Task.Success, output.(string))
		if err != nil {
			return err
		}
		// copy the data over
		if this.Task.Output != nil {
			err = zk.CreateOrSetString(this.zk, *this.Task.Output, output.(string))
			if err != nil {
				return err
			}
			this.Log("Success", "Result written to", this.Task.Output.Path())
		}
	default:
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
	}
	this.Log("Success", "Result written to", this.Task.Success.Path())

	now := time.Now()
	this.Stat.Success = &now
	err := zk.CreateOrSet(this.zk, this.Info, this.Stat)
	if err != nil {
		return err
	}

	this.Log("Success", "Completed")
	this.Stop()
	return nil
}

func (this *Runtime) Error(error interface{}) error {
	if this.done {
		return ErrStopped
	}
	switch error.(type) {
	case []byte:
		err := zk.CreateOrSetBytes(this.zk, this.Task.Error, error.([]byte))
		if err != nil {
			return err
		}
	case string:
		err := zk.CreateOrSetString(this.zk, this.Task.Error, error.(string))
		if err != nil {
			return err
		}
	default:
		value, err := json.Marshal(error)
		if err != nil {
			return err
		}
		err = zk.CreateOrSetBytes(this.zk, this.Task.Error, value)
		if err != nil {
			return err
		}
	}
	this.Log("Error", "Error written to", this.Task.Error.Path())

	now := time.Now()
	this.Stat.Error = &now
	err := zk.CreateOrSet(this.zk, this.Info, this.Stat)
	if err != nil {
		return err
	}

	this.Log("Error", "Stop")
	this.Stop()
	return nil
}
