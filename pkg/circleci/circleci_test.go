package circleci

import (
	"fmt"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"os"
	"testing"
)

func TestCircleCI(t *testing.T) { TestingT(t) }

type CircleCITests struct{}

var _ = Suite(&CircleCITests{})

const token = "76681eca1d76e43f6535589def6756a27723d8e0"

const linux_build = "linux_amd64"
const macosx_build = "darwin_386"

func (suite *CircleCITests) TestFetchBuildArtifacts(c *C) {

	filter, err := MatchPathAndBinary(macosx_build, "passport")
	c.Assert(err, Equals, nil)

	cl := &Build{
		User:     "qorio",
		Project:  "omni",
		ApiToken: token,
	}

	var build int = 291
	artifacts, err := cl.FetchBuildArtifacts(build, filter)
	c.Assert(err, Equals, nil)
	c.Assert(len(artifacts), Equals, 1)
	c.Log("build", artifacts)

	temp_dir, err := ioutil.TempDir("", fmt.Sprintf("build-%d-", build))
	c.Assert(err, Equals, nil)

	for _, a := range artifacts {
		len, err := a.Download(temp_dir)
		c.Log("name=", a.Name, ",len=", len, ",dir=", temp_dir, ",err=", err)

		test, err := os.Open(temp_dir + "/" + a.Name)
		c.Assert(err, Equals, nil)
		fi, err := test.Stat()
		c.Assert(err, Equals, nil)
		c.Assert(fi.Size(), Not(Equals), 0)
		c.Assert(fi.IsDir(), Equals, false)
		c.Assert(fi.Mode()|0111, Not(Equals), 0)
	}

	// clean up
	os.RemoveAll(temp_dir)
}
