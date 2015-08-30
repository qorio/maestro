package circleci

import (
	. "gopkg.in/check.v1"
	"testing"
)

func TestBuild(t *testing.T) { TestingT(t) }

type BuildTests struct{}

var _ = Suite(&BuildTests{})

func (suite *BuildTests) TestBuild(c *C) {
	yml := new(CircleYml)
	err := yml.LoadFromBytes([]byte(`
## Circle CI configuration
machine:
  services:
    - docker

  timezone:
    America/Los_Angeles

  # Override /etc/hosts
  hosts:
    circlehost: 127.0.0.1

  # Add some environment variables
  environment:
    GOPATH: $HOME/go
    PATH: $GOPATH/bin:$PATH
    CIRCLE_ENV: test
    DOCKER_ACCOUNT: infradash
    DOCKER_EMAIL: docker@infradash.com
    BUILD_LABEL: $CIRCLE_BUILD_NUM
    BUILD_DIR: build/bin

## Customize dependencies
dependencies:
  pre:
    - go version
  override:
    - echo $BUILD_LABEL and gopath=$GOPATH

## Customize test commands
test:
  override:
    - echo "Running tests."
    - echo $DOCKER_ACCOUNT and email = $DOCKER_EMAIL

## Customize deployment commands
deployment:
   git:
     branch: /release\/.*/
     commands:
       - echo $BUILD_DIR/dash $CIRCLE_ARTIFACTS

   docker:
     branch: /v[0-9]+(\.[0-9]+)*/
     commands:
       - echo $BUILD_DIR/dash $CIRCLE_BUILD_NUM
`))

	c.Assert(err, Equals, nil)
	c.Log(yml)

	b := &Build{
		ProjectUser:  "david",
		Project:      "test",
		GitBranch:    "v1.0",
		BuildNum:     10,
		ArtifactsDir: "/dev/null",
		User:         "tester",
	}

	err = b.Build(yml)
	c.Assert(err, Equals, nil)
}
