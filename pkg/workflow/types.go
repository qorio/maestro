package workflow

import (
	"fmt"
	"time"
)

type Path string

type Orchestration struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Input       Path   `json:"input,omitempty"`
	Output      Path   `json:"output,omitempty"`

	Tasks map[TaskName]Task `json:"tasks" yaml:"tasks"`
}

type TaskName string
type Task struct {
	StartTrigger Path          `json:"start,omitempty"`
	Condition    *Condition    `json:"condition,omitempty"`
	WorkerPolicy *WorkerPolicy `json:"workers,omitempty"`
	Success      Path          `json:"success,omitempty"`
	Error        Path          `json:"error,omitempty"`

	Scheduler Reference `json:"scheduler,omitempty"`
}

// { singleton | scheduler | barrier-N | hostname: }
type WorkerPolicy string

type Reference string

type Timeout time.Duration

type Condition struct {
	Timeout      *Timeout `json:"timeout,omitempty"`
	PathExists   *Path    `json:"path_exists,omitempty"`
	PathChildren *Path    `json:"path_children"`
	MinChildren  int      `json:"min_children"`
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
