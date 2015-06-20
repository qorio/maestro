package task

import (
	"bytes"
	"encoding/gob"
	"text/template"
)

/// Makes a deep copy
func (this *Cmd) Copy() (*Cmd, error) {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	dec := gob.NewDecoder(&buff)
	err := enc.Encode(this)
	if err != nil {
		return nil, err
	}
	copy := new(Cmd)
	err = dec.Decode(copy)
	if err != nil {
		return nil, err
	}
	return copy, nil
}

func apply(s string, ctx map[string]interface{}, funcs template.FuncMap) (string, error) {
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
func (this *Cmd) ApplySubstitutions(env map[string]interface{}, funcs template.FuncMap) (*Cmd, error) {

	applied, err := this.Copy()
	if err != nil {
		return nil, err
	}

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

	for i, arg := range applied.Args {
		if sub, err := apply(arg, env, funcs); err != nil {
			return nil, err
		} else {
			applied.Args[i] = sub
		}
	}

	for i, kv := range applied.Env {
		if sub, err := apply(kv, env, funcs); err != nil {
			return nil, err
		} else {
			applied.Env[i] = sub
		}
	}
	return applied, nil
}
