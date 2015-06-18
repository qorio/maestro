package task

import (
	"bytes"
	"text/template"
)

func apply(s string, ctx map[string]string, funcs template.FuncMap) (string, error) {
	t, err := template.New(s).Parse(s)
	if funcs != nil {
		t.Funcs(funcs)
	}
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	err = t.Execute(&buff, ctx)
	if err != nil {
		return "", err
	} else {
		return buff.String(), nil
	}
}

// Given the environments apply any substitutions to the command and args
// the format is same as template expressions e.g. {{.RUN_BINARY}}
// This allows the environment to be passed to the command even if the child process
// does not look at environment variables.
func (this *Cmd) ApplySubstitutions(env map[string]string, funcs template.FuncMap) (*Cmd, error) {

	applied := *this // first copy

	if sub, err := apply(this.Dir, env, nil); err != nil {
		return nil, err
	} else {
		applied.Dir = sub
	}

	if sub, err := apply(this.Path, env, nil); err != nil {
		return nil, err
	} else {
		applied.Path = sub
	}

	for i, arg := range this.Args {
		applied.Args = make([]string, len(this.Args))
		if sub, err := apply(arg, env, funcs); err != nil {
			return nil, err
		} else {
			applied.Args[i] = sub
		}
	}

	list := []string{}
	for k, v := range env {
		list = append(list, k+"="+v)
	}
	applied.Env = list

	return &applied, nil
}
