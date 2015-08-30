package circleci

import ()

const CircleApiPrefix = "https://circleci.com/api/v1"

type Config struct {
	User     string `json:"user"`
	Project  string `json:"project"`
	ApiToken string `json:"token"`
}

type BuildArtifact struct {
	Path       string `json:"path,omitempty"`
	PrettyPath string `json:"pretty_path,omitempty"`
	URL        string `json:"url,omitempty"`
	Name       string `json:"name,omitempty"`
	circleci   *Config
}

// https://circleci.com/docs/environment-variables
/*
CIRCLE_PROJECT_USERNAME
The username or organization name of the project being tested, i.e. "foo" in circleci.com/gh/foo/bar/123
CIRCLE_PROJECT_REPONAME
The repository name of the project being tested, i.e. "bar" in circleci.com/gh/foo/bar/123
CIRCLE_BRANCH
The name of the branch being tested, e.g. 'master'.
CIRCLE_SHA1
The SHA1 of the commit being tested.
CIRCLE_COMPARE_URL
A link to GitHub's comparison view for this push. Not present for builds that are triggered by GitHub pushes.
CIRCLE_BUILD_NUM
The build number, same as in circleci.com/gh/foo/bar/123
CIRCLE_PREVIOUS_BUILD_NUM
The build number of the previous build, same as in circleci.com/gh/foo/bar/123
CI_PULL_REQUESTS
Comma-separated list of pull requests this build is a part of.
CI_PULL_REQUEST
If this build is part of only one pull request, its URL will be populated here. If there was more than one pull request, it will contain one of the pull request URLs (picked randomly).
CIRCLE_ARTIFACTS
The directory whose contents are automatically saved as build artifacts.
CIRCLE_USERNAME
The GitHub login of the user who either pushed the code to GitHub or triggered the build from the UI/API.
CIRCLE_TEST_REPORTS
The directory whose contents are automatically processed as JUnit test metadata.
*/
type Build struct {
	Project          string `json:"project"`
	ProjectUser      string `json:"project_user"`
	User             string `json:"user"`
	Token            string `json:"token"`
	BuildNum         int    `json:"build_num"`
	PreviousBuildNum int    `json:"previous_build_num"`
	GitRepo          string `json:"git_repo"`
	GitBranch        string `json:"git_branch"`
	Commit           string `json:"commit"`
	ArtifactsDir     string `json:"artifacts_dir"`

	yml *CircleYml
}

type CircleYml struct {
	Machine      Machine `yaml:"machine,omitempty"`
	Dependencies Block   `yaml:"dependencies,omitempty"`
	Test         Block   `yaml:"test,omitempty"`
	Deployment   Targets `yaml:"deployment,omitempty"`
}

type Machine struct {
	Services    []string       `yaml:"services,omitempty"`
	Timezone    string         `yaml:"timezone,omitempty"`
	Hosts       HostMap        `yaml:"hosts,omitempty"`
	Environment EnvironmentMap `yaml:"environment,omitempty"`
}

type HostMap map[string]string
type EnvironmentMap map[string]string

type Block struct {
	Pre      []string `yaml:"pre,omitempty"`
	Override []string `yaml:"override,omitempty"`
}

type Targets map[string]Deployment

type Deployment struct {
	Branch   string   `yaml:"branch,omitempty"`
	Commands []string `yaml:"commands,omitempty"`
}
