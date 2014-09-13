package yaml

import (
	"errors"
	"fmt"
	"github.com/qorio/maestro/pkg/circleci"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (this *Image) Validate(c Context) error {
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

	api := circleci.Config{
		User:     parts[0],
		Project:  parts[1],
		ApiToken: c["CIRCLECI_API_TOKEN"].(string),
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
	return nil
}

func (this *Image) Execute(c Context) error {
	return nil
}

func (this *Image) Finish(c Context) error {
	return nil
}
