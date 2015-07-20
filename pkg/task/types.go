package task

import (
	"errors"
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/registry"
	"time"
)

var (
	ErrCommandUnknown = errors.New("command-unknown")
	ErrExecFailed     = errors.New("exec-failed")
)

type Orchestration struct {
	Id          string            `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Label       string            `json:"label,omitempty"`
	Description string            `json:"description,omitempty"`
	Log         pubsub.Topic      `json:"log,omitempty"`
	StartTime   *time.Time        `json:"start_time,omitempty"`
	Context     registry.Path     `json:"context,omitempty"`
	Tasks       map[TaskName]Task `json:"tasks,omitempty"`
}

type CronExpression string

// TODO -- add ordering.  Cron before Registry, or Registry activates cron
type Trigger struct {
	Cron     *CronExpression      `json:"cron,omitempty"`
	Registry *registry.Conditions `json:"registry,omitempty"`
}

type Cmd struct {
	Dir  string   `json:"working_dir,omitempty"`
	Path string   `json:"path"`
	Args []string `json:"args"`
	Env  []string `json:"env"`
}

type TaskName string
type Task struct {
	// Required
	Id   string   `json:"id"`
	Name TaskName `json:"name"`

	Info    registry.Path `json:"info"`
	Success registry.Path `json:"success"`
	Error   registry.Path `json:"error"`

	Context *registry.Path `json:"context"`

	// Conditional execution
	Trigger *Trigger `json:"trigger,omitempty"`

	// Topics (e.g. mqtt://localhost:1281/aws-cli/124/stdout)
	Status pubsub.Topic  `json:"status"`
	Stdin  *pubsub.Topic `json:"stdin,omitempty"`
	Stdout *pubsub.Topic `json:"stdout,omitempty"`
	Stderr *pubsub.Topic `json:"stderr,omitempty"`

	Cmd  *Cmd `json:"cmd,omitempty"`
	Runs int  `json:"runs,omitempty"`

	Stat TaskStat
}

// Written to the Info path of the task
type TaskStat struct {
	Started   *time.Time `json:"started,omitempty"`
	Triggered *time.Time `json:"triggered,omitempty"`
	Success   *time.Time `json:"success,omitempty"`
	Error     *time.Time `json:"error,omitempty"`
}
