package docker

import (
	"os"
	"text/template"
)

const docker_cfg_template = `
{"https://index.docker.io/v1/":{"auth":"{{.Auth}}","email":"{{.Email}}"}}
`

type Config struct {
	Email string
	Auth  string
}

func (this *Config) GenerateDockerCfg(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	t := template.Must(template.New("dockercfg").Parse(docker_cfg_template))
	err = t.Execute(file, this)
	if err != nil {
		return err
	}
	err = file.Chmod(0600)
	if err != nil {
		return err
	}
	return nil
}
