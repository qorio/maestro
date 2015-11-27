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

// type Orchestration struct {
// 	Id          string            `json:"id,omitempty"`
// 	Name        string            `json:"name,omitempty"`
// 	Label       string            `json:"label,omitempty"`
// 	Description string            `json:"description,omitempty"`
// 	Log         pubsub.Topic      `json:"log,omitempty"`
// 	StartTime   *time.Time        `json:"start_time,omitempty"`
// 	Context     registry.Path     `json:"context,omitempty"`
// 	Tasks       map[TaskName]Task `json:"tasks,omitempty"`
// }

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

type Announce struct {
	Key       string
	Value     interface{}
	Ephemeral bool
}

type TaskName string
type Task struct {
	// Required
	Id  string `json:"id,omitempty"`
	Cmd *Cmd   `json:"cmd,omitempty"`

	// If this is set to true then we only require id and command to be set
	ExecOnly bool `json:"exec_only"`

	Name TaskName `json:"name"`

	// Optional namespace for task related announcements in the regstry.
	AnnounceNamespace *registry.Path `json:"announce,omitempty"`

	// Optional registry paths to set success / failure signals
	Info    registry.Path `json:"info,omitempty"`
	Success registry.Path `json:"success,omitempty"`
	Error   registry.Path `json:"error,omitempty"`

	// Conditional execution
	Trigger *Trigger `json:"trigger,omitempty"`

	// Topics (e.g. mqtt://localhost:1281/aws-cli/124/stdout)
	Status pubsub.Topic  `json:"status"`
	Stdin  *pubsub.Topic `json:"stdin,omitempty"`
	Stdout *pubsub.Topic `json:"stdout,omitempty"`
	Stderr *pubsub.Topic `json:"stderr,omitempty"`

	Runs int `json:"runs,omitempty"`

	Stats TaskStats `json:"stats,omitempty"`

	PrintPre        string `json:"print_pre,omitempty"`
	PrintPost       string `json:"print_post,omitempty"`
	PrintErr        string `json:"print_err,omitempty"`
	PrintErrWarning bool   `json:"print_err_warning,omitempty"`
}

// Written to the Info path of the task
type TaskStats struct {
	Started   *time.Time `json:"started,omitempty"`
	Triggered *time.Time `json:"triggered,omitempty"`
	Success   *time.Time `json:"success,omitempty"`
	Error     *time.Time `json:"error,omitempty"`
}
