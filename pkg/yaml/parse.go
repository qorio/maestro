package yaml

import (
	"errors"
	"fmt"
	mapset "github.com/deckarep/golang-set"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var SizeQuantityUnitRegex *Regexp

type Regexp struct {
	*regexp.Regexp
}

func (r *Regexp) Parse(s string) map[string]string {
	captures := make(map[string]string)

	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}

	for i, name := range r.SubexpNames() {
		if i == 0 {
			continue
		}
		captures[name] = match[i]
	}
	return captures
}

const (
	kb = 1
	mb = 1000 * kb
	gb = 1000 * mb
	tb = 1000 * gb
)

type SizeUnit int64

const (
	KB SizeUnit = kb
	MB SizeUnit = mb
	GB SizeUnit = gb
	TB SizeUnit = tb
)

func init() {
	SizeQuantityUnitRegex = &Regexp{regexp.MustCompile(`(?P<quantity>\d+)(?P<unit>\[K|k|M|m|G|g|T|t])`)}
}

func (this SizeQuantityUnit) Validate() error {
	m := SizeQuantityUnitRegex.Parse(string(this))
	if len(m) == 0 {
		return errors.New(fmt.Sprintf("bad-size-quntity-unit:%s", this))
	}
	_, err := strconv.ParseFloat(m["quantity"], 64)
	if err != nil {
		return errors.New(fmt.Sprintf("bad-size-quntity-unit:%s", this))
	}
	return nil
}

func (this SizeQuantityUnit) ToUnit(su SizeUnit) float64 {
	m := SizeQuantityUnitRegex.Parse(string(this))
	if len(m) == 0 {
		return -1
	}
	q, err := strconv.ParseFloat(m["quantity"], 64)
	if err != nil {
		return -1
	}

	unit := float64(su)
	switch strings.ToLower(m["unit"]) {
	case "k":
		return q * kb / unit
	case "m":
		return q * mb / unit
	case "g":
		return q * gb / unit
	case "t":
		return q * tb / unit
	}
	return -1
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
