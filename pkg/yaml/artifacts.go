package yaml

import (
	"errors"
	"github.com/qorio/maestro/pkg/circleci"
	"log"
	"strconv"
	"strings"
)

func (this *Artifact) circleci(c Context) (*circleci.Config, int64, error) {
	parts := strings.Split(this.Project, "/")
	if len(parts) != 2 {
		return nil, 0, errors.New("Project not in format of <user>/<proj>: " + this.Project)
	}

	api := circleci.Config{
		User:     parts[0],
		Project:  parts[1],
		ApiToken: this.SourceApiToken,
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
	c.eval(&this.SourceApiToken)

	filter, err := circleci.MatchPathAndBinary(this.Platform, string(this.name))
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
	dir := c["binary_dir"]
	api, build, err := this.circleci(c)
	if err != nil {
		return err
	}
	filter, err := circleci.MatchPathAndBinary(this.Platform, string(this.name))
	if err != nil {
		return err
	}
	binaries, err := api.FetchBuildArtifacts(build, filter)
	if err != nil {
		return err
	}
	if len(binaries) == 0 {
		return errors.New("Binary for " + string(this.name) + " not found on " + this.Source)
	}
	log.Println("Downloading binary", this.name, "build", build, "to", dir)
	bytes, err := binaries[0].Download(dir.(string))
	if err != nil {
		log.Println("error", err)
		return err
	}
	log.Println(bytes, "bytes")
	return nil
}
