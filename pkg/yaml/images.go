package yaml

import (
	"errors"
	"fmt"
	"github.com/qorio/maestro/pkg/circleci"
	"github.com/qorio/maestro/pkg/docker"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const CIRCLECI_API_TOKEN = "CIRCLECI_API_TOKEN"
const DOCKER_EMAIL = "DOCKER_EMAIL"
const DOCKER_AUTH = "DOCKER_AUTH"
const DOCKER_ACCOUNT = "DOCKER_ACCOUNT"
const TEST_MODE = "TEST_MODE"

func (this *Image) Validate(c Context) error {
	// Check required vars
	if _, has := c[DOCKER_EMAIL]; !has {
		return errors.New("Missing DOCKER_EMAIL var")
	}
	if _, has := c[DOCKER_AUTH]; !has {
		return errors.New("Missing DOCKER_AUTH var")
	}

	c.eval(&this.Dockerfile)
	c.eval(&this.RepoId)

	if len(this.artifacts) == 0 && this.RepoId == "" {
		return errors.New("No artifacts reference to build this image or no Docker hub repo id specified.")
	}

	for _, artifact := range this.artifacts {
		log.Println("Validating asset", artifact.Name)
		if err := artifact.Validate(c); err != nil {
			return err
		}
	}

	// check to see if docker file exists.
	fi, err := os.Stat(this.Dockerfile)
	if err != nil {
		return errors.New(fmt.Sprint("Missing dockerfile at", this.Dockerfile, ":", err))
	}
	if fi.IsDir() {
		return errors.New(fmt.Sprint("Dockerfile", this.Dockerfile, "is a directory."))
	}

	return nil
}

func (this *Artifact) circleci(c Context) (*circleci.Config, int64, error) {
	parts := strings.Split(this.Project, "/")
	if len(parts) != 2 {
		return nil, 0, errors.New("Project not in format of <user>/<proj>: " + this.Project)
	}

	token, ok := c[CIRCLECI_API_TOKEN].(string)
	if !ok {
		return nil, 0, errors.New("CIRCLECI_API_TOKEN not a string.")
	}

	api := circleci.Config{
		User:     parts[0],
		Project:  parts[1],
		ApiToken: token,
	}
	build, err := strconv.ParseInt(this.BuildNumber, 10, 64)
	if err != nil {
		return nil, 0, errors.New("Must be a numeric build number")
	}
	return &api, build, nil
}

func (this *Artifact) Validate(c Context) error {
	// Apply the variables to all the string fields since they can reference variables
	c.eval(&this.Project)
	c.eval(&this.Source)
	c.eval(&this.BuildNumber)
	c.eval(&this.Artifact)
	c.eval(&this.Platform)

	filter, err := circleci.MatchPathAndBinary(this.Platform, string(this.Name))
	if err != nil {
		return err
	}
	// Currently only support circleci
	switch this.Source {
	case "circleci":
		if _, has := c["CIRCLECI_API_TOKEN"]; !has {
			return errors.New("CIRCLECI_API_TOKEN var is missing")
		} else {
			api, build, err := this.circleci(c)
			if err != nil {
				return err
			}
			log.Println("Checking availability of", this.Name, ", build", build)
			binaries, err := api.FetchBuildArtifacts(build, filter)
			if err != nil {
				return err
			}
			if len(binaries) == 0 {
				return errors.New("Binary for " + string(this.Name) + " not found on " + this.Source)
			} else {
				log.Println("Found binary for", this.Name, "from", this.Source, "path=", binaries[0].Path)
			}
		}
	default:
		return errors.New("Source " + this.Source + " not supported.")
	}
	return nil
}

func (this *Image) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Artifact) Prepare(c Context) error {
	dir := c["binary_dir"]
	api, build, err := this.circleci(c)
	if err != nil {
		return err
	}
	filter, err := circleci.MatchPathAndBinary(this.Platform, string(this.Name))
	if err != nil {
		return err
	}
	binaries, err := api.FetchBuildArtifacts(build, filter)
	if err != nil {
		return err
	}
	if len(binaries) == 0 {
		return errors.New("Binary for " + string(this.Name) + " not found on " + this.Source)
	}
	log.Println("Downloading binary", this.Name, "build", build, "to", dir)
	bytes, err := binaries[0].Download(dir.(string))
	if err != nil {
		log.Println("error", err)
		return err
	}
	log.Println(bytes, "bytes")
	return nil
}

func docker_config(c Context) (*docker.Config, error) {
	email, ok := c[DOCKER_EMAIL].(string)
	if !ok {
		return nil, errors.New("DOCKER_EMAIL not a string")
	}
	auth, ok := c[DOCKER_AUTH].(string)
	if !ok {
		return nil, errors.New("DOCKER_AUTH not a string")
	}

	account, ok := c[DOCKER_ACCOUNT].(string)
	if !ok {
		return nil, errors.New("DOCKER_ACCOUNT not a string")
	}

	return &docker.Config{
		Email:   email,
		Auth:    auth,
		Account: account,
	}, nil
}

func (this *Image) Prepare(c Context) error {
	// for each artifact, pull the binary and place in the dockerfile's directory
	dir := filepath.Dir(this.Dockerfile)
	c["binary_dir"] = dir
	defer delete(c, "binary_dir")

	for _, artifact := range this.artifacts {
		err := artifact.Prepare(c)
		if err != nil {
			return err
		}
	}

	// set up dockercfg file
	log.Println("Setting up .dockercfg")
	f := filepath.Join(os.Getenv("HOME"), ".dockercfg")
	fi, err := os.Stat(f)
	switch {
	case err == nil && fi.IsDir():
		return errors.New("~/.dockercfg is a directory.")
	case err == nil: // overwrite
	case os.IsNotExist(err): // no file
		docker_config, err := docker_config(c)
		if err != nil {
			return nil
		}
		err = docker_config.GenerateDockerCfg(f)
		if err != nil {
			return err
		}
		log.Println("Created dockercfg.")
	default:
		return err
	}

	return nil
}

func (this *Image) Execute(c Context) error {

	docker_config, err := docker_config(c)
	if err != nil {
		return nil
	}
	_, docker_config.TestMode = c[TEST_MODE]

	image, err := docker_config.NewTaggedImage(this.RepoId, this.Dockerfile)
	if err != nil {
		return err
	}

	err = image.Build()
	if err != nil {
		return err
	}

	log.Println("Finished building", this.Name, "Now pushing.")

	err = image.Push()
	if err != nil {
		return err
	}

	return nil
}

func (this *Image) Finish(c Context) error {
	return nil
}
