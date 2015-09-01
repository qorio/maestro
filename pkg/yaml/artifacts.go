package yaml

import (
	"errors"
	"fmt"
	"github.com/qorio/maestro/pkg/circleci"
	"log"
	"strconv"
	"strings"
)

func (this *Artifact) circleci(c Context) (*circleci.Build, int, error) {
	parts := strings.Split(this.Project, "/")
	if len(parts) != 2 {
		return nil, 0, errors.New("Project not in format of <user>/<proj>: " + this.Project)
	}

	api := circleci.Build{
		User:     parts[0],
		Project:  parts[1],
		ApiToken: this.SourceApiToken,
	}
	build, err := strconv.ParseInt(this.BuildNumber, 10, 64)
	if err != nil {
		return nil, 0, errors.New("Must be a numeric build number:" + this.BuildNumber)
	}
	return &api, int(build), nil
}

func (this *Artifact) get_circleci_lookup_filter() (circleci.BuildArtifactFilter, error) {
	filter, err := circleci.MatchPathAndBinary(this.Platform, this.File)
	if err != nil {
		return nil, err
	}
	return filter, nil
}

func (this *Artifact) Validate(c Context) error {
	_, err := strconv.ParseInt(this.BuildNumber, 10, 64)
	if err != nil {
		return errors.New("Must be a numeric build number:" + this.BuildNumber)
	}

	filter, err := this.get_circleci_lookup_filter()
	if err != nil {
		return err
	}

	// Currently only support circleci
	switch this.Source {
	case "circleci":
		if this.SourceApiToken == "" {
			return errors.New("CIRCLECI requires source-api-token to be set.")
		} else {
			api, build, err := this.circleci(c)
			if err != nil {
				return err
			}
			log.Println("Checking availability of", this.name, ", build", build)
			binaries, err := api.FetchBuildArtifacts(build, filter)
			if err != nil {
				return err
			}
			if len(binaries) == 0 {
				return errors.New("Binary for " + string(this.name) + " not found on " + this.Source)
			} else {
				log.Println("Found binary for", this.name, "from", this.Source, "path=", binaries[0].Path)
			}
		}
	default:
		return errors.New("Source " + this.Source + " not supported.")
	}
	return nil
}

func (this *Artifact) Prepare(c Context) error {
	return nil
}

func (this *Artifact) Execute(c Context) error {
	dir := c["binary_dir"]
	if dir == nil {
		return errors.New(fmt.Sprintf("no-directory-to-save-download-artifact:%s", this.name))
	}

	api, build, err := this.circleci(c)
	if err != nil {
		return err
	}
	filter, err := this.get_circleci_lookup_filter()
	if err != nil {
		return err
	}

	binaries, err := api.FetchBuildArtifacts(build, filter)
	if err != nil {
		return err
	}
	if len(binaries) == 0 {
		return errors.New("Binary for " + string(this.name) + " (" + this.File + ") not found on " + this.Source)
	}
	c.log("Artifact[%s] -- Downloading binary %s, build %d, to %s", this.name, this.File, build, dir)
	if c.test_mode() {
		return nil
	}

	bytes, err := binaries[0].Download(dir.(string))
	if err != nil {
		c.error("Artifact[%s] -- Error: %s", this.name, err.Error())
		return err
	}
	c.log("Artifact[%s] -- Downloading %d bytes (binary %s, build %d, in %s)", this.name, bytes, this.File, build, dir)
	return nil
}

func (this *Artifact) Finish(c Context) error {
	return nil
}

func (this *Artifact) get_task() *task {
	if this.task == nil {
		this.task = alloc_task(this)
		this.task.description = fmt.Sprintf("Artifact[%s]", this.name)
	}
	return this.task
}
