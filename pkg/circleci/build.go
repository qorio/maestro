package circleci

import (
	"fmt"
	"github.com/golang/glog"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func (this *Build) ExportEnvironments() error {

	os.Setenv("CIRCLE_PROJECT_USERNAME", this.ProjectUser)
	os.Setenv("CIRCLE_PROJECT_REPONAME", this.Project)
	os.Setenv("CIRCLE_REPO_URL", this.GitRepo)
	os.Setenv("CIRCLE_BRANCH", this.GitBranch)
	os.Setenv("CIRCLE_SHA1", this.Commit)
	os.Setenv("CIRCLE_BUILD_NUM", fmt.Sprintf("%d", this.BuildNum))
	os.Setenv("CIRCLE_PREVIOUS_BULID_NUM", fmt.Sprintf("%d", this.PreviousBuildNum))
	os.Setenv("CIRCLE_ARTIFACTS", this.ArtifactsDir)
	os.Setenv("CIRCLE_USERNAME", this.User)

	if this.yml == nil {
		return nil
	}

	for k, v := range this.yml.Machine.Environment {
		// expand and export
		vv := os.ExpandEnv(v)
		os.Setenv(k, vv)
	}
	return nil
}

func original(original []string) []string {
	return original
}

func expand_envs(original []string) []string {
	result := []string{}
	for _, s := range original {
		result = append(result, os.ExpandEnv(s))
	}
	return result
}

func (this *Build) Build(yml *CircleYml) error {
	this.yml = yml
	if this.yml == nil {
		return nil
	}
	if this.LogStart == nil {
		this.LogStart = func(p Phase) {}
	}
	if this.LogEnd == nil {
		this.LogEnd = func(Phase, error) bool { return true }
	}

	if err := this.ExportEnvironments(); err != nil {
		return err
	}

	filter := original

	var err error = nil

	////////////////
	this.LogStart(PhaseDependencies)
	for _, line := range filter(this.yml.Dependencies.Pre) {
		if err = execute(line); err != nil {
			break
		}
	}
	for _, line := range filter(this.yml.Dependencies.Override) {
		if err = execute(line); err != nil {
			break
		}
	}
	if !this.LogEnd(PhaseDependencies, err) {
		return err
	}

	////////////////
	this.LogStart(PhaseTest)
	for _, line := range filter(this.yml.Test.Pre) {
		if err = execute(line); err != nil {
			break
		}
	}
	for _, line := range filter(this.yml.Test.Override) {
		if err = execute(line); err != nil {
			break
		}
	}
	if !this.LogEnd(PhaseTest, err) {
		return err
	}

	////////////////
	this.LogStart(PhaseDeployment)
	for k, deploy := range this.yml.Deployment {
		pat := deploy.Branch
		match := false
		if strings.Index(pat, "/") == 0 && strings.LastIndex(pat, "/") == len(pat)-1 {
			pat = pat[1 : len(pat)-1]
			match, _ = regexp.Match(pat, []byte(this.GitBranch))
		} else {
			match = this.GitBranch == deploy.Branch
		}
		if match {

			for _, line := range filter(deploy.Commands) {
				if err = execute(line); err != nil {
					break
				}
			}
		} else {
			glog.Infoln("Skipping", k, deploy.Branch)
		}
	}
	this.LogEnd(PhaseDeployment, err)
	return err
}

func execute(line string) error {
	c := exec.Command("sh", "-")
	if stdout, err := c.StdoutPipe(); err == nil {
		go func() {
			io.Copy(os.Stdout, stdout)
		}()
	} else {
		return err
	}

	if stderr, err := c.StderrPipe(); err == nil {
		go func() {
			io.Copy(os.Stderr, stderr)
		}()
	} else {
		return err
	}

	stdin, err := c.StdinPipe()
	if err != nil {
		return err
	}

	if err := c.Start(); err != nil {
		return err
	}
	glog.Infoln("start:", line)

	if _, err := stdin.Write([]byte(line)); err != nil {
		stdin.Close()
		return err
	}
	stdin.Close() // finished
	err = c.Wait()

	if ee, ok := err.(*exec.ExitError); ok {
		glog.Infoln("PID", ee.Pid(), " - Process state", ee.Success())
	}
	return err
}
