package workflow

import (
	"fmt"
	"github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/maestro/pkg/registry"
	"time"
)

type Orchestration struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	Tasks map[TaskName]Task `json:"tasks"`
}

type TaskName string
type Task struct {
	// Required
	Info    registry.Path `json:"info"`
	Success registry.Path `json:"success"`
	Error   registry.Path `json:"error"`
	Status  pubsub.Topic  `json:"status_topic"`

	// Triggering
	StartTrigger *registry.Path `json:"start,omitempty"`
	Condition    *Condition     `json:"condition,omitempty"`
	WorkerPolicy *WorkerPolicy  `json:"workers,omitempty"`

	// registry.Paths for storing input/output
	Input  *registry.Path `json:"input,omitempty"`
	Output *registry.Path `json:"output,omitempty"`

	// Topics (e.g. mqtt://aws-cli/124/stdout)
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

type Timeout time.Duration

type Condition struct {
	Timeout     *Timeout       `json:"timeout,omitempty"`
	Exists      *registry.Path `json:"exists,omitempty"`
	Changes     *registry.Path `json:"changes,omitempty"`
	MinChildren int            `json:"min_children"`
}

func (this *Timeout) UnmarshalJSON(s []byte) error {
	// unquote the string
	d, err := time.ParseDuration(string(s[1 : len(s)-1]))
	if err != nil {
		return err
	}
	*this = Timeout(d)
	return nil
}

func (this *Timeout) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", time.Duration(*this).String())), nil
}
