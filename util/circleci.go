package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type CircleCI struct {
	User     string
	Project  string
	ApiToken string
}

type BuildArtifact struct {
	Path       string `json:"path,omitempty"`
	PrettyPath string `json:"pretty_path,omitempty"`
	URL        string `json:"url,omitempty"`
	Name       string `json:"name,omitempty"`
	circleci   *CircleCI
}

const CircleApiPrefix = "https://circleci.com/api/v1"

func (this *CircleCI) url(format string, parts ...interface{}) (*url.URL, error) {
	url_main := CircleApiPrefix + fmt.Sprintf(format, parts...)
	url, err := url.Parse(url_main)
	if err != nil {
		return nil, err
	}
	q := url.Query()
	q.Add("circle-token", this.ApiToken)
	url.RawQuery = q.Encode()
	log.Println("URL", url)
	return url, nil
}

type BuildArtifactFilter func(*BuildArtifact) bool

func (this *CircleCI) FetchBuildArtifacts(buildNum int, filter BuildArtifactFilter) ([]BuildArtifact, error) {
	url, err := this.url("/project/%s/%s/%d/artifacts", this.User, this.Project, buildNum)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	get, err := http.NewRequest("GET", url.String(), nil)
	get.Header.Add("Accept", "application/json")
	log.Println("About to get", url.String())
	resp, err := client.Do(get)
	if err != nil {
		return nil, err
	}

	artifacts := []BuildArtifact{}
	err = json.NewDecoder(resp.Body).Decode(&artifacts)
	if err != nil {
		return nil, err
	}

	result := make([]BuildArtifact, 0)
	for _, a := range artifacts {
		accept := true
		if filter != nil {
			accept = filter(&a)
		}
		if accept {
			a.Name = filepath.Base(a.Path) // parse the path for the name of the binary
			a.circleci = this
			result = append(result, a)
		}
	}
	return result, nil
}

// Downloads the artifact to the directory specified
func (this *BuildArtifact) Download(dir string) (int64, error) {
	if this.URL == "" {
		return 0, errors.New("no-url")
	}
	client := &http.Client{}
	get, err := http.NewRequest("GET", this.URL+"?circle-token="+this.circleci.ApiToken, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(get)
	if err != nil {
		return 0, err
	}

	// make directory if necessary
	err = os.MkdirAll(dir, 0777)
	if err != nil {
		return 0, err
	}

	file, err := os.Create(filepath.Join(dir, this.Name))
	if err != nil {
		return 0, err
	}

	err = file.Chmod(0555)
	if err != nil {
		return 0, err
	}
	return io.Copy(file, resp.Body)
}
