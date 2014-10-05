package yaml

import (
	mapset "github.com/deckarep/golang-set"
	"log"
	"sort"
	"strconv"
	"strings"
)

func (this runnableMap) Validate(c Context) error {
	return this.apply_sequential("VALIDATE", c, func(cc Context, rr Runnable) error {
		return rr.Finish(cc)
	})
}

func (this runnableMap) Prepare(c Context) error {
	return this.apply_sequential("PREPARE", c, func(cc Context, rr Runnable) error {
		return rr.Prepare(cc)
	})
}

func (this runnableMap) Execute(c Context) error {
	return this.apply_sequential("EXECUTE", c, func(cc Context, rr Runnable) error {
		return rr.Execute(cc)
	})
}

func (this runnableMap) Finish(c Context) error {
	return this.apply_sequential("FINISH", c, func(cc Context, rr Runnable) error {
		return rr.Finish(cc)
	})
}

func (this runnableMap) apply_sequential(phase string, c Context, f func(Context, Runnable) error) error {
	for k, runnable := range this {
		log.Println(phase, ":", k)
		err := f(c, runnable)
		if err != nil {
			return err
		}
	}
	return nil
}

func (this JobPortList) parse() ([]ExposedPort, error) {
	tokens := strings.Split(string(this), ",")
	sort.Strings(tokens)
	ports := make([]ExposedPort, len(tokens))
	for i, tok := range tokens {
		if p, err := strconv.ParseInt(strings.TrimSpace(tok), 10, 64); err != nil {
			return nil, err
		} else {
			ports[i] = ExposedPort(p)
		}
	}
	return ports, nil
}

func (this InstanceLabelList) parse() []InstanceLabel {
	tokens := strings.Split(string(this), ",")
	sort.Strings(tokens)
	labels := make([]InstanceLabel, len(tokens))
	for i, tok := range tokens {
		labels[i] = InstanceLabel(strings.TrimSpace(tok))
	}
	return labels
}

func new_set(this []InstanceLabel) mapset.Set {
	set := mapset.NewSet()
	for _, v := range this {
		set.Add(v)
	}
	return set
}

func intersect(this, other []InstanceLabel) bool {
	return new_set(this).Intersect(new_set(other)).Cardinality() > 0
}
