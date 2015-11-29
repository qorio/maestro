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
	"text/template"
	"time"
)

var (
	ErrBadConfig            = errors.New("bad-config")
	ErrBadConfigInfo        = errors.New("bad-config-info")
	ErrBadConfigStatus      = errors.New("bad-config-status")
	ErrBadConfigSuccess     = errors.New("bad-config-success")
	ErrBadConfigError       = errors.New("bad-config-error")
	ErrBadConfigCmdNotFound = errors.New("bad-config-cmd-not-found")

	ErrStopped = errors.New("stopped")
	ErrTimeout = errors.New("timeout")

	stop_announce = Announce{Key: "stop"}
)

type Runtime struct {
	Task

	TimestampStart int64
	TimestampExit  int64

	zk zk.ZK

	announce chan Announce
	status   chan []byte
	stdout   chan []byte
	stderr   chan []byte
	stdin    chan []byte

	options interface{}
	done    bool
	ready   bool
	lock    sync.Mutex
	error   error

	stdoutBuff       *bytes.Buffer
	stdinInterceptor func(string) (string, bool)

	Status string
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
	if this.ExecOnly {
		switch {
		case this.Id == "":
			return ErrBadConfig
		case this.Cmd == nil:
			return ErrBadConfig
		default:
			_, err := exec.LookPath(this.Cmd.Path)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// If we are in Orchestration mode then a lot more needs to be set
	switch {
	case this.Namespace != nil && !this.Namespace.Valid():
		return ErrBadConfigInfo
	case !this.LogTopic.Valid():
		return ErrBadConfigStatus
	case this.Success != nil && !this.Success.Valid():
		return ErrBadConfigSuccess
	case this.Error != nil && !this.Error.Valid():
		return ErrBadConfigError
	case this.Cmd != nil:
		_, err := exec.LookPath(this.Cmd.Path)
		if err != nil {
			return ErrBadConfigCmdNotFound
		}
	}

	if err := parse_template(this.LogTemplateStart, &this.templateStart); err != nil {
		return err
	}
	if err := parse_template(this.LogTemplateStop, &this.templateStop); err != nil {
		return err
	}
	if err := parse_template(this.LogTemplateSuccess, &this.templateSuccess); err != nil {
		return err
	}
	if err := parse_template(this.LogTemplateError, &this.templateError); err != nil {
		return err
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

	if task.announce == nil {
		task.announce = make(chan Announce, 10)
	}

	if task.Task.Stdout != nil {
		task.stdout = make(chan []byte)
	}

	if task.Task.Stderr != nil {
		task.stderr = make(chan []byte)
	}

	// Default interceptor
	task.stdinInterceptor = func(in string) (string, bool) {
		return in, strings.Index(in, "#bye") != 0
	}

	task.Stats.Started = time.Now().Unix()

	task.set_defaults()
	task.start_announcer()

	return &task, nil
}

func (this *Runtime) Stop() {

	this.Log(this.build_message(this.templateStop)...)

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
	this.status <- nil
	this.done = true
}

func (this *Runtime) Announce() chan<- Announce {
	return this.announce
}

func (this *Runtime) StdinInterceptor(f func(string) (string, bool)) {
	this.stdinInterceptor = f
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
			glog.Fatalln("Error getting stdout.", "Topic=", *this.Task.Stdout, "Err=", err)
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
		glog.Fatalln("Error getting stderr.", "Topic=", *this.Task.Stderr, "Err=", err)
		return nil
	}
}

func (this *Runtime) Log(m ...interface{}) {
	switch {
	case len(m) == 0:
		return
	case this.done:
		return
	}

	source := ""
	_, file, line, ok := runtime.Caller(1)
	if ok {
		source = fmt.Sprintf("%s:%d", file, line)
	}

	msg := fmt.Sprint(m...)
	this.status <- []byte(msg)
	glog.Infoln(source, m)
}

func (this *Runtime) Running() bool {
	return !this.done
}

func (this *Runtime) ApplyEnvAndFuncs(env map[string]interface{}, funcs map[string]interface{}) error {
	if this.Task.Cmd == nil {
		return nil
	}

	this.lock.Lock()
	defer this.lock.Unlock()

	applied, err := this.Task.Cmd.ApplySubstitutions(env, funcs)
	if err != nil {
		return err
	}
	this.Task.Cmd = applied
	return nil
}

func (this *Runtime) set_defaults() {
	if len(this.LogTopic) > 0 {
		if this.Task.Stdout == nil {
			t := this.LogTopic.Sub("stdout")
			this.Task.Stdout = &t
		}
		if this.Task.Stderr == nil {
			t := this.LogTopic.Sub("stderr")
			this.Task.Stderr = &t
		}
	}
}

func parse_template(t *string, tt **template.Template) error {
	if t == nil {
		return nil
	}
	p, err := template.New(*t).Parse(*t)
	if err != nil {
		return err
	}
	*tt = p
	return nil
}

func (this *Runtime) build_message(t *template.Template) []interface{} {
	if t == nil {
		return []interface{}{}
	}
	var buff bytes.Buffer
	err := t.Execute(&buff, map[string]interface{}{
		"id":     this.Task.Id,
		"name":   this.Task.Name,
		"start":  this.TimestampStart,
		"exit":   this.TimestampExit,
		"status": this.Status,
	})
	if err != nil {
		return []interface{}{}
	}
	return []interface{}{buff.String()}
}

func (this *Runtime) Start() (chan error, error) {
	this.TimestampStart = this.Now()
	this.Status = fmt.Sprint("Starting ", this.Task.Cmd.Path)

	go func() {
		for {
			m := <-this.status
			if m == nil {
				break
			}
			if c, err := this.Task.LogTopic.Broker().PubSub(this.Id, this.options); err == nil {
				c.Publish(this.Task.LogTopic, m)
			} else {
				glog.Warningln("Cannot publish:", this.Task.LogTopic.String(), "Err=", err)
			}
		}
	}()

	this.Log(this.build_message(this.templateStart)...)

	if _, _, err := this.start_streams(); err != nil {
		return nil, err
	}

	if err := this.block_on_triggers(); err == zk.ErrTimeout {
		return nil, ErrTimeout
	}

	// Run the actual task
	if this.Task.Cmd != nil {
		return this.exec()
	}
	return nil, nil
}

func (this *Runtime) Now() int64 {
	return time.Now().Unix()
}

// Announcer will publish to registry at the client's request.
func (this *Runtime) start_announcer() {
	switch {
	case this.zk == nil, this.Namespace == nil:
		glog.Infoln("No zk or no announcement namespace defined.  Not announcing.")
		go func() {
			for {
				if a := <-this.announce; a == stop_announce {
					glog.Infoln("Stopping announcer")
					break
				} else {
					glog.Infoln("Skipping announcement:", a)
				}
			}
		}()
	default:
		go func() {
			for {
				if a := <-this.announce; a == stop_announce {
					glog.Infoln("Stopping announcer")
					break
				} else {
					key := a.Key
					if len(key) == 0 {
						key = fmt.Sprintf("%d", time.Now().Unix())
					}

					path := this.Namespace.Sub(key)
					var err error
					switch a.Value.(type) {
					case []byte:
						err = zk.CreateOrSetBytes(this.zk, path, a.Value.([]byte), a.Ephemeral)
					case string:
						err = zk.CreateOrSetString(this.zk, path, a.Value.(string), a.Ephemeral)
					default:
						if value, err := json.Marshal(a.Value); err == nil {
							err = zk.CreateOrSetBytes(this.zk, path, value, a.Ephemeral)
						}
					}

					if err != nil {
						this.Log("Cannot annouce to", path.Path(), "Err=", err.Error())
					} else {
						this.Log("ANNOUNCE", path.Path())
					}
				}
			}
		}()
	}
}

func (this *Runtime) block_on_triggers() error {
	if this.Cmd == nil {
		return nil
	}

	if this.Trigger == nil {
		return nil
	}

	// TODO - take into account ordering of cron vs registry.

	if this.Trigger.Registry != nil {
		trigger := zk.NewConditions(*this.Trigger.Registry, this.zk)
		// So now just block until the condition is true
		this.Log("Waiting for trigger.")
		return trigger.Wait()
	}

	return nil
}

func (this *Runtime) exec() (chan error, error) {
	cmd := exec.Command(this.Cmd.Path, this.Cmd.Args...)
	cmd.Dir = this.Cmd.Dir
	cmd.Env = this.Cmd.Env

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
				if l, ok := this.stdinInterceptor(string(m)); ok {
					fmt.Printf(">> %s", l)
					wr.Write([]byte(l))
				} else {
					wr.Close()
					return
				}
			}
		}()
	}
	cmd.Stdout = this.Stdout()
	cmd.Stderr = this.Stderr()

	process_done := make(chan error)
	go func() {
		cmd.Start()
		this.Status = "Started."

		// Wait for cmd to complete even if we have no more stdout/stderr
		if err := cmd.Wait(); err != nil {
			this.Status = err.Error()
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

		this.Status = fmt.Sprint("Process pid=", ps.Pid(), "Exited=", ps.Exited(), "Success=", ps.Success())
		glog.Infoln(this.Status)

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
	if this.stdout != nil {
		glog.Infoln("Starting stream for stdout:", this.Task.Stdout.String())
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
		glog.Infoln("Starting stream for stderr:", this.Task.Stderr.String())
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
	defer this.Log(this.build_message(this.templateSuccess)...)

	if this.zk == nil {
		glog.Infoln("Not connected to zk.  Output not recorded")
		return nil
	}
	if this.done {
		return ErrStopped
	}

	if this.Task.Success != nil {
		switch output.(type) {
		case []byte:
			err := zk.CreateOrSetBytes(this.zk, *this.Task.Success, output.([]byte))
			if err != nil {
				return err
			}
		case string:
			err := zk.CreateOrSetString(this.zk, *this.Task.Success, output.(string))
			if err != nil {
				return err
			}
		default:
			value, err := json.Marshal(output)
			if err != nil {
				return err
			}
			err = zk.CreateOrSetBytes(this.zk, *this.Task.Success, value)
			if err != nil {
				return err
			}
		}
		this.Log("Success", "Result written to", this.Task.Success.Path())
	}

	now := this.Now()
	this.Stats.Success = now
	this.TimestampExit = now

	this.Announce() <- Announce{
		Key:       "exit",
		Value:     this.Stats,
		Ephemeral: false,
	}
	this.Log("Success", "Completed")
	return nil
}

func (this *Runtime) Error(error interface{}) error {
	defer this.Log(this.build_message(this.templateError)...)

	if this.zk == nil {
		glog.Infoln("Not connected to zk.  Output not recorded")
		return nil
	}
	if this.done {
		return ErrStopped
	}

	if this.Task.Error != nil {
		switch error.(type) {
		case []byte:
			err := zk.CreateOrSetBytes(this.zk, *this.Task.Error, error.([]byte))
			if err != nil {
				return err
			}
		case string:
			err := zk.CreateOrSetString(this.zk, *this.Task.Error, error.(string))
			if err != nil {
				return err
			}
		default:
			value, err := json.Marshal(error)
			if err != nil {
				return err
			}
			err = zk.CreateOrSetBytes(this.zk, *this.Task.Error, value)
			if err != nil {
				return err
			}
		}
		this.Log("Error", "Error written to", this.Task.Error.Path())
	}

	now := this.Now()
	this.Stats.Error = now
	this.TimestampExit = now

	this.Announce() <- Announce{
		Key:       "exit",
		Value:     this.Stats,
		Ephemeral: false,
	}
	this.Log("Error", "Stop")
	return nil
}
