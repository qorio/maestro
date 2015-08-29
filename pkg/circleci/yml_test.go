package circleci

import (
	. "gopkg.in/check.v1"
	"testing"
)

func TestCircleYml(t *testing.T) { TestingT(t) }

type CircleYmlTests struct{}

var _ = Suite(&CircleYmlTests{})

func (suite *CircleYmlTests) TestParseYml(c *C) {
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
    # Set up authentication to Docker Registry
    - sed "s/<EMAIL>/$DOCKER_EMAIL/;s/<AUTH>/$DOCKER_AUTH/" < ./docker/dockercfg.template > ~/.dockercfg
  override:
    - source ./hack/env.sh

## Customize test commands
test:
  override:
    - echo "Running tests."
    - godep go test ./pkg/... -v -check.vv -logtostderr

## Customize deployment commands
deployment:
   git:
     branch: /release\/.*/
     commands:
       - source ./hack/env.sh && make GODEP=godep build
       - cp $BUILD_DIR/dash $CIRCLE_ARTIFACTS
       - source ./hack/env.sh && make deploy-git

   docker:
     branch: /v[0-9]+(\.[0-9]+)*/
     commands:
       - source ./hack/env.sh && make GODEP=godep build
       - cp $BUILD_DIR/dash $CIRCLE_ARTIFACTS
       - source ./hack/env.sh && make deploy-git-docker-image-version
       - cp $BUILD_DIR/dash docker/dash
       - cd docker/dash && make push && cd ..
`))

	c.Assert(err, Equals, nil)
	c.Log(yml)

	c.Assert(yml.Deployment["git"], DeepEquals, Deployment{
		Branch: "/release\\/.*/",
		Commands: []string{
			"source ./hack/env.sh && make GODEP=godep build",
			"cp $BUILD_DIR/dash $CIRCLE_ARTIFACTS",
			"source ./hack/env.sh && make deploy-git",
		},
	})

	c.Assert(yml.Test.Pre, DeepEquals, []string(nil))
	c.Assert(yml.Test.Override, DeepEquals, []string{
		"echo \"Running tests.\"",
		"godep go test ./pkg/... -v -check.vv -logtostderr",
	})

	c.Assert(yml.Machine.Environment["GOPATH"], Equals, "$HOME/go")
	c.Assert(yml.Machine.Environment["BUILD_LABEL"], Equals, "$CIRCLE_BUILD_NUM")

	buff, err := yml.AsYml()
	c.Assert(err, Equals, nil)
	c.Log(string(buff))
}
