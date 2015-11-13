package registry

import (
	"fmt"
	"path"
	"strings"
	"time"
)

type Path string

func NewPath(s string, parts ...string) Path {
	return Path(path.Join("/", s, path.Join(parts...)))
}

func (this Path) Sub(parts ...string) Path {
	return Path(path.Join(string(this), path.Join(parts...)))
}

func (this Path) Base() string {
	return path.Base(string(this))
}

func (this Path) Dir() Path {
	return Path(path.Dir(string(this)))
}

func (this Path) Parts() []string {
	return strings.Split(string(this), "/")
}

type Timeout time.Duration

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

type Conditions struct {
	Timeout *Timeout `json:"timeout,omitempty"`

	Create  *Create  `json:"create,omitempty"`  // if node created
	Delete  *Delete  `json:"delete,omitempty"`  // if node deleted
	Change  *Change  `json:"change,omitempty"`  // if node value changed
	Members *Members `json:"members,omitempty"` // if node members changed

	// Default is ANY of the condition met will fire.
	All bool `json:"all,omitempty"`
}

type Key interface {
	Valid() bool
	Path() string
}

func (p Path) Member(k string) Path {
	return Path(string(p) + "/" + k)
}

type Delete Path
type Create Path
type Change Path
type Members struct {
	Top          Path   `json:"path"`
	Min          *int32 `json:"min,omitempty"`
	Max          *int32 `json:"max,omitempty"`
	Equals       *int32 `json:"eq,omitempty"`
	OutsideRange bool   `json:"outside_range,omitempty"` // default is within range.  true for outside range.
	Delta        *int32 `json:"delta,omitempty"`         // delta of count
}

func (p Path) Path() string {
	return string(p)
}

func (p Path) Valid() bool {
	return len(p) > 0
}

func (d Delete) Path() string {
	return string(d)
}

func (d Delete) Valid() bool {
	return len(d) > 0
}

func (e Create) Path() string {
	return string(e)
}

func (e Create) Valid() bool {
	return len(e) > 0
}

func (c Change) Path() string {
	return string(c)
}

func (c Change) Valid() bool {
	return len(c) > 0
}

func (m Members) Path() string {
	return m.Top.Path()
}

func (m Members) Valid() bool {
	if m.Top.Valid() {
		// Ok: x > Min, x < Max, x in [Max, Min], x == Equals
		switch {
		case m.Min != nil && m.Equals != nil:
			return false
		case m.Max != nil && m.Equals != nil:
			return false
		case m.Equals == nil && m.Max == nil && m.Min == nil:
			return false
		}
	}
	return false
}
