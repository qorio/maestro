package yaml

import (
	"errors"
	"fmt"
	"github.com/qorio/maestro/pkg/docker"
	"os"
	"path/filepath"
)

const DOCKER_EMAIL = "DOCKER_EMAIL"
const DOCKER_AUTH = "DOCKER_AUTH"
const DOCKER_ACCOUNT = "DOCKER_ACCOUNT"

func (this *Image) export_vars() map[string]interface{} {
	artifacts := make(map[string]map[string]interface{})
	for _, a := range this.artifacts {
		artifacts[string(a.name)] = map[string]interface{}{
			"project":  a.Project,
			"source":   a.Source,
			"build":    a.BuildNumber,
			"file":     a.File,
			"platform": a.Platform,
		}
	}

	return map[string]interface{}{
		"id":         this.RepoId,
		"dockerfile": this.Dockerfile,
		"name":       this.name,
		"artifacts":  artifacts,
	}
}

func (this *Image) Validate(c Context) error {
	// Check required vars
	if _, has := c[DOCKER_EMAIL]; !has {
		return errors.New("Missing DOCKER_EMAIL var")
	}
	if _, has := c[DOCKER_AUTH]; !has {
		return errors.New("Missing DOCKER_AUTH var")
	}

	if len(this.artifacts) == 0 && this.RepoId == "" {
		return errors.New("No artifacts reference to build this image or no Docker hub repo id specified.")
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

func (this *Image) InDesiredState(c Context) (bool, error) {
	return true, nil
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

	// set up dockercfg file

	c.log("Setting up .dockercfg")

	if c.test_mode() {
		return nil
	}

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
		c.log("Created dockercfg.")
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
	docker_config.TestMode = c.test_mode()

	c.log("Building docker image from Dockerfile %s", this.Dockerfile)

	image, err := docker_config.NewTaggedImage(this.RepoId, this.Dockerfile)
	if err != nil {
		return err
	}

	err = image.Build()
	if err != nil {
		return err
	}

	c.log("Finished building %s. Now pushing", this.name)

	err = image.Push()
	if err != nil {
		return err
	}

	return nil
}

func (this *Image) Finish(c Context) error {
	return nil
}

func (this *Image) get_task() *task {
	if this.task == nil {
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Image[%s]", this.name)
		// Assume artifacts are already built and made available so we can parallelize fetch
		for _, artifact := range this.artifacts {
			this.task.DependsOn(artifact.get_task())
		}
	}
	return this.task
}
