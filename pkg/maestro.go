package util

import (
	_ "gopkg.in/yaml.v1"
)

type ServiceSpec struct {
	CircleCI    string
	BuildNumber int
}
