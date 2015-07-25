package common

import (
	"fmt"
	"net/url"
)

type Url url.URL

func NewUrl(s string) *Url {
	u, err := url.Parse(s)
	if err != nil {
		return nil
	}
	uu := Url(*u)
	return &uu
}

func (u *Url) UnmarshalJSON(ss []byte) error {
	parsed, err := url.Parse(string(ss[1 : len(ss)-1]))
	if err != nil {
		return err
	}

	u.Scheme = parsed.Scheme
	u.Opaque = parsed.Opaque
	u.User = parsed.User
	u.Host = parsed.Host
	u.Path = parsed.Path
	u.RawQuery = parsed.RawQuery
	u.Fragment = parsed.Fragment
	return nil
}

func (u *Url) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", u.String())), nil
}

func (u *Url) String() string {
	return (*url.URL)(u).String()
}
