package task

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/zk"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrBadConfig = errors.New("bad-config")
	ErrStopped   = errors.New("stopped")
	ErrTimeout   = errors.New("timeout")
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
	lock    sync.Mutex
	error   error

	stdoutBuff *bytes.Buffer
}

func (this *Task) Copy() (*Task, error) {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	dec := gob.NewDecoder(&buff)
	err := enc.Encode(this)
	if err != nil {
		return nil, err
	}
	copy := new(Task)
	err = dec.Decode(copy)
	if err != nil {
		return nil, err
	}
	return copy, nil
}

func (this *Task) Validate() error {
	switch {
	case !this.Info.Valid():
		return ErrBadConfig
	case !this.Success.Valid():
		return ErrBadConfig
	case !this.Error.Valid():
		return ErrBadConfig
	case this.Exec != nil:
		_, err := exec.LookPath(this.Exec.Path)
		if err != nil {
			return err
		}
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

	if task.zk != nil {
		err := zk.CreateOrSet(task.zk, task.Info, task.Stat)
		if err != nil {
			return nil, err
		}
	}
	return &task, nil
}

func (this *Runtime) Stop() {
	this.lock.Lock()
	defer this.lock.Unlock()

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
		return os.Stdin
	}
	if c, err := this.Task.Stdin.Broker().PubSub(this.Id, this.options); err == nil {
		return pubsub.GetReader(*this.Task.Stdin, c)
	} else {
		return nil
	}
}

func (this *Runtime) PublishStdin() io.Writer {
	if this.Task.Stdin == nil {
		return os.Stdin
	}
	if c, err := this.Task.Stdin.Broker().PubSub(this.Id, this.options); err == nil {
		return pubsub.GetWriter(*this.Task.Stdin, c)
	} else {
		glog.Warningln("Error getting stdin.", "Topic=", *this.Task.Stdin, "Err=", err)
		return nil
	}
}

func (this *Runtime) CaptureStdout() {
	this.stdoutBuff = new(bytes.Buffer)
}

func (this *Runtime) GetCapturedStdout() []byte {
	if this.stdoutBuff != nil {
		return this.stdoutBuff.Bytes()
	}
	return nil
}

func (this *Runtime) Stdout() io.Writer {
	var stdout io.Writer = os.Stdout
	if this.Task.Stdout != nil {
		if c, err := this.Task.Stdout.Broker().PubSub(this.Id, this.options); err == nil {
			stdout = pubsub.GetWriter(*this.Task.Stdout, c)
		} else {
			glog.Warningln("Error getting stdout.", "Topic=", *this.Task.Stdout, "Err=", err)
			return nil
		}
	}

	if this.stdoutBuff != nil {
		stdout = io.MultiWriter(stdout, this.stdoutBuff)
	}
	return stdout
}

func (this *Runtime) Stderr() io.Writer {
	if this.Task.Stderr == nil {
		return os.Stderr
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

func (this *Runtime) ApplyEnvAndFuncs(env map[string]interface{}, funcs map[string]interface{}) error {
	if this.Task.Exec == nil {
		return nil
	}

	this.lock.Lock()
	defer this.lock.Unlock()

	applied, err := this.Task.Exec.ApplySubstitutions(env, funcs)
	if err != nil {
		return err
	}
	this.Task.Exec = applied
	return nil
}

func (this *Runtime) Start() (chan error, error) {
	_, _, err := this.start_streams()
	if err != nil {
		return nil, err
	}

	err = this.block_on_triggers()
	if err == zk.ErrTimeout {
		return nil, ErrTimeout
	}

	// Run the actual task
	if this.Task.Exec != nil {
		return this.exec()
	}
	return nil, nil
}

func (this *Runtime) block_on_triggers() error {
	if this.Start == nil {
		return nil
	}

	if this.Trigger == nil {
		return nil
	}

	// TODO - take into account ordering of cron vs registry.

	if this.Trigger.Registry != nil {
		trigger := zk.NewConditions(*this.Trigger.Registry, this.zk)
		// So now just block until the condition is true
		return trigger.Wait()
	}

	return nil
}

func (this *Runtime) exec() (chan error, error) {
	cmd := exec.Command(this.Exec.Path, this.Exec.Args...)
	cmd.Dir = this.Exec.Dir
	cmd.Env = this.Exec.Env

	if this.Task.Stdin != nil {
		sub, err := this.Task.Stdin.Broker().PubSub(this.Id, this.options)
		if err != nil {
			return nil, err
		}
		stdin, err := sub.Subscribe(*this.Task.Stdin)
		if err != nil {
			return nil, err
		}
		wr, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		go func() {
			// We need to do some special processing of input so that we can
			// terminate a session. Otherwise, this will just loop forever
			// because the pubsub topic will not go away -- even if it's a unique topic.
			for {
				m := <-stdin
				fmt.Printf(">> %s", string(m))
				switch {
				case bytes.Equal(m, []byte("@bye")):
					wr.Close()
					return
				default:
					wr.Write(m) // Need newline for shell to interpret
				}
			}
		}()
	}
	cmd.Stdout = this.Stdout()
	cmd.Stderr = this.Stderr()

	process_done := make(chan error)
	go func() {
		cmd.Start()

		// Wait for cmd to complete even if we have no more stdout/stderr
		if err := cmd.Wait(); err != nil {
			this.Error(err.Error())
			process_done <- err
			return
		}

		ps := cmd.ProcessState
		if ps == nil {
			this.Error(ErrCommandUnknown.Error())
			process_done <- ErrCommandUnknown
			return
		}

		glog.Infoln("Process pid=", ps.Pid(), "Exited=", ps.Exited(), "Success=", ps.Success())

		if !ps.Success() {
			this.Error(ErrExecFailed.Error())
			process_done <- ErrExecFailed
			return
		} else {
			this.Success(this.GetCapturedStdout())
			process_done <- nil
			return
		}
	}()

	return process_done, nil
}

func (this *Runtime) start_streams() (stdout, stderr chan<- []byte, err error) {
	this.lock.Lock()
	defer func() {
		this.error = err
		this.lock.Unlock()
	}()

	if this.error != nil {
		return nil, nil, this.error
	}

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
	if this.zk == nil {
		glog.Infoln("Not connected to zk.  Output not recorded")
		this.Stop()
		return nil
	}

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
	if this.zk == nil {
		glog.Infoln("Not connected to zk.  Output not recorded")
		this.Stop()
		return nil
	}

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
