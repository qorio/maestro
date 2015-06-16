package workflow

import (
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/registry"
	"time"
)

type Orchestration struct {
	Id          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	Tasks map[TaskName]Task `json:"tasks"`
}

type TaskName string
type Task struct {
	// Required
	Id      string        `json:"id"`
	Info    registry.Path `json:"info"`
	Success registry.Path `json:"success"`
	Error   registry.Path `json:"error"`
	Status  pubsub.Topic  `json:"status"`

	// Triggering
	StartTrigger *registry.Conditions `json:"conditions,omitempty"`
	WorkerPolicy *WorkerPolicy        `json:"workers,omitempty"`

	// registry.Paths for storing input/output
	Input  *registry.Path `json:"input,omitempty"`
	Output *registry.Path `json:"output,omitempty"`

	// Topics (e.g. mqtt://localhost:1281/aws-cli/124/stdout)
	Stdin  *pubsub.Topic `json:"stdin_topic,omitempty"`
	Stdout *pubsub.Topic `json:"stdout_topic,omitempty"`
	Stderr *pubsub.Topic `json:"stderr_topic,omitempty"`

	Scheduler Reference `json:"scheduler,omitempty"`

	Stat TaskStat
}

// Written to the Info path of the task
type TaskStat struct {
	Started   *time.Time `json:"started,omitempty"`
	Triggered *time.Time `json:"triggered,omitempty"`
	Success   *time.Time `json:"success,omitempty"`
	Error     *time.Time `json:"error,omitempty"`
}

// { singleton | scheduler | barrier-N | hostname: }
type WorkerPolicy string

type Reference string
