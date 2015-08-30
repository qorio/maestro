package circleci

import (
	"encoding/json"
	"gopkg.in/yaml.v1"
	"io"
	"io/ioutil"
)

func (this *CircleYml) AsJSON() ([]byte, error) {
	return json.Marshal(this)
}

func (this *CircleYml) AsYml() ([]byte, error) {
	return yaml.Marshal(this)
}

func (this *CircleYml) LoadFromReader(file io.Reader) error {
	buff, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	return this.LoadFromBytes(buff)
}

func (this *CircleYml) LoadFromBytes(buff []byte) error {
	return yaml.Unmarshal(buff, this)
}
