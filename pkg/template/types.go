package template

import (
	"errors"
)

var (
	ErrNotSupportedProtocol = errors.New("protocol-not-supported")
	ErrNotConnectedToZk     = errors.New("not-connected-to-zk")
	ErrMissingTemplateFunc  = errors.New("no-template-func")
	ErrBadTemplateFunc      = errors.New("err-bad-template-func")
)
