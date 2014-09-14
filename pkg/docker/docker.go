package docker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const docker_cfg_template = `
{"https://index.docker.io/v1/":{"auth":"{{.Auth}}","email":"{{.Email}}"}}
`

type Config struct {
	Email    string
	Auth     string
	Account  string
	TestMode bool
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

type docker_build struct {
	repo       string
	build      int64
	tag        string
	dockerFile string
	dockerDir  string
	config     *Config
	cmd        *exec.Cmd
}

func (this *Config) NewImage(repo string, build int64, dockerFile string) (*docker_build, error) {
	if this.Account == "" {
		return nil, errors.New("Missing docker hub account.")
	}
	fi, err := os.Stat(dockerFile)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, errors.New("Dockerfile is a directory:" + dockerFile)
	}
	return &docker_build{
		repo:       repo,
		build:      build,
		tag:        fmt.Sprintf("%s/%s:%d", this.Account, repo, build),
		dockerFile: dockerFile,
		dockerDir:  filepath.Dir(dockerFile),
		config:     this,
	}, nil
}

func (this *Config) NewTaggedImage(tag string, dockerFile string) (*docker_build, error) {
	if this.Account == "" {
		return nil, errors.New("Missing docker hub account.")
	}
	fi, err := os.Stat(dockerFile)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, errors.New("Dockerfile is a directory:" + dockerFile)
	}
	return &docker_build{
		tag:        tag,
		dockerFile: dockerFile,
		dockerDir:  filepath.Dir(dockerFile),
		config:     this,
	}, nil
}

func (this *docker_build) Cmd() *exec.Cmd {
	return this.cmd
}

func watch(in io.ReadCloser) {
	defer in.Close()
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return
	}
}

func watch_cmd(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	go watch(stdout)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go watch(stderr)

	return nil
}

func (this *docker_build) Build() error {
	cmd := exec.Command("docker", "build", "-t", this.tag, this.dockerDir)
	if this.config.TestMode {
		cmd = exec.Command("echo", "docker", "build", "-t", this.tag, this.dockerDir)
	}

	err := watch_cmd(cmd)
	if err != nil {
		return err
	}

	log.Println("Executing", cmd.Path, cmd.Args, "dir=", cmd.Dir)
	cmd.Start()
	if err := cmd.Wait(); err != nil {
		log.Println("Error:", err)
		return err
	}
	return nil
}

func (this *docker_build) Push() error {
	cmd := exec.Command("docker", "push", this.tag)
	if this.config.TestMode {
		cmd = exec.Command("echo", "docker", "push", this.tag)
	}

	err := watch_cmd(cmd)
	if err != nil {
		return err
	}

	log.Println("Executing", cmd.Path, cmd.Args, "dir=", cmd.Dir)
	cmd.Start()
	if err := cmd.Wait(); err != nil {
		log.Println("Error:", err)
		return err
	}
	return nil
}
