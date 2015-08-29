package circleci

import (
	"encoding/json"
	"gopkg.in/yaml.v1"
	"io"
	"io/ioutil"
)

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

func (this *CircleYml) AsJSON() ([]byte, error) {
	return json.Marshal(this)
}

func (this *CircleYml) AsYml() ([]byte, error) {
	return yaml.Marshal(this)
}

func (this *CircleYml) LoadFromFile(file io.Reader) error {
	buff, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	return this.LoadFromBytes(buff)
}

func (this *CircleYml) LoadFromBytes(buff []byte) error {
	return yaml.Unmarshal(buff, this)
}
