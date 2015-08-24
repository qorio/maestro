package docker

import (
	"bufio"
	"bytes"
	_docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"io"
	"net"
	"path"
	"strings"
	"time"
)

type Docker struct {
	Endpoint string

	Cert string
	Key  string
	Ca   string

	docker *_docker.Client

	ContainerCreated func(*Container)
	ContainerStarted func(*Container)
}

type Port struct {
	ContainerPort int64  `json:"container_port"`
	HostPort      int64  `json:"host_port"`
	Type          string `json:"protocol"`
	AcceptIP      string `json:"accepts_ip"`
}

type Container struct {
	Id      string `json:"id"`
	Ip      string `json:"ip"`
	Image   string `json:"image"`
	ImageId string `json:"image_id"`

	Name    string `json:"name"`
	Command string `json:"command"`
	Ports   []Port `json:"ports"`
	Network _docker.NetworkSettings

	DockerData *_docker.Container `json:"docker_data"`

	docker *_docker.Client
}

type AuthIdentity struct {
	_docker.AuthConfiguration
}

type Image struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

func (this Image) ImageString() string {
	s := this.Repository
	if this.Tag != "" {
		s = s + ":" + this.Tag
	}
	return s
}

func (this Image) Url() string {
	return path.Join(this.Registry, this.ImageString())
}

func ParseImageUrl(url string) Image {
	image := Image{}
	delim1 := strings.Index(url, "://")
	if delim1 < 0 {
		delim1 = 0
	} else {
		delim1 += 3
	}
	tag_index := strings.LastIndex(url[delim1:], ":")
	if tag_index > -1 {
		tag_index += delim1
		image.Tag = url[tag_index+1:]
	} else {
		tag_index = len(url)
	}
	project := path.Base(url[0:tag_index])
	account := path.Base(path.Dir(url[0:tag_index]))
	delim2 := strings.Index(url, account)
	image.Registry = url[0 : delim2-1]
	image.Repository = path.Join(account, project)
	return image
}

type ContainerControl struct {
	*_docker.Config

	// If false, the container starts up in daemon mode (as a service) - default
	RunOnce       bool                `json:"run_once,omitempty"`
	HostConfig    *_docker.HostConfig `json:"host_config"`
	ContainerName string              `json:"name,omitempty"`
}

// Endpoint and file paths
func NewTLSClient(endpoint string, cert, key, ca string) (c *Docker, err error) {
	c = &Docker{Endpoint: endpoint, Cert: cert, Ca: ca, Key: key}
	c.docker, err = _docker.NewTLSClient(endpoint, cert, key, ca)
	return c, err
}

func NewClient(endpoint string) (c *Docker, err error) {
	c = &Docker{Endpoint: endpoint}
	c.docker, err = _docker.NewClient(endpoint)
	return c, err
}

func (c *Docker) ListContainers() ([]*Container, error) {
	return c.FindContainers(nil)
}

func (c *Docker) FindContainersByName(name string) ([]*Container, error) {
	found := make([]*Container, 0)
	l, err := c.FindContainers(map[string][]string{
		"name": []string{name},
	})
	if err != nil {
		return nil, err
	}
	for _, cc := range l {
		err := cc.Inspect() // populates the Name, etc.
		glog.V(100).Infoln("Inspect container", *cc, "Err=", err)
		if err == nil && cc.Name == name {
			found = append(found, cc)
		}
	}
	return found, nil
}

func (c *Docker) FindContainers(filter map[string][]string) ([]*Container, error) {
	options := _docker.ListContainersOptions{
		All:  true,
		Size: true,
	}
	if filter != nil {
		options.Filters = filter
	}
	l, err := c.docker.ListContainers(options)
	if err != nil {
		return nil, err
	}
	out := []*Container{}
	for _, cc := range l {

		glog.V(100).Infoln("Matching", options, "Container==>", cc.Ports)
		c := &Container{
			Id:      cc.ID,
			Image:   cc.Image,
			Command: cc.Command,
			Ports:   get_ports(cc.Ports),
			docker:  c.docker,
		}
		c.Inspect()
		out = append(out, c)
	}
	return out, nil
}

func (c *Docker) PullImage(auth *AuthIdentity, image *Image) (<-chan error, error) {
	output_buff := bytes.NewBuffer(make([]byte, 1024*4))
	output := bufio.NewWriter(output_buff)

	err := c.docker.PullImage(_docker.PullImageOptions{
		Repository:   image.Repository,
		Registry:     image.Registry,
		Tag:          image.Tag,
		OutputStream: output,
	}, auth.AuthConfiguration)

	if err != nil {
		return nil, err
	}

	// Since the api doesn't have a channel, all we can do is read from the input
	// and then send a done signal when the input stream is exhausted.
	stopped := make(chan error)
	go func() {
		for {
			_, e := output_buff.ReadByte()
			if e == io.EOF {
				stopped <- nil
				return
			} else {
				stopped <- e
				return
			}
		}
	}()
	return stopped, err
}

func (c *Docker) StartContainer(auth *AuthIdentity, ct *ContainerControl) (*Container, error) {
	opts := _docker.CreateContainerOptions{
		Name:       ct.ContainerName,
		Config:     ct.Config,
		HostConfig: ct.HostConfig,
	}

	daemon := !ct.RunOnce
	// Detach mode (-d option in docker run)
	if daemon {
		opts.Config.AttachStdin = false
		opts.Config.AttachStdout = false
		opts.Config.AttachStderr = false
		opts.Config.StdinOnce = false
	}

	cc, err := c.docker.CreateContainer(opts)
	if err != nil {
		return nil, err
	}

	container := &Container{
		Id:     cc.ID,
		Image:  ct.Image,
		docker: c.docker,
	}

	if c.ContainerCreated != nil {
		c.ContainerCreated(container)
	}

	err = c.docker.StartContainer(cc.ID, ct.HostConfig)
	if err != nil {
		return nil, err
	}

	if c.ContainerStarted != nil {
		c.ContainerStarted(container)
	}

	err = container.Inspect()
	return container, err
}

func (c *Docker) StopContainer(auth *AuthIdentity, id string, timeout time.Duration) error {
	return c.docker.StopContainer(id, uint(timeout.Seconds()))
}

func (c *Docker) RemoveContainer(auth *AuthIdentity, id string, removeVolumes, force bool) error {
	return c.docker.RemoveContainer(_docker.RemoveContainerOptions{
		ID:            id,
		RemoveVolumes: removeVolumes,
		Force:         force,
	})
}

type Action int

const (
	Create Action = iota
	Start
	Stop
	Remove
	Die
)

// Docker event status are create -> start -> die -> stop for a container then destroy for docker -rm
var verbs map[string]Action = map[string]Action{
	"create":  Create,
	"start":   Start,
	"stop":    Stop,
	"destroy": Remove,
	"die":     Die,
}

func (c *Docker) WatchContainer(notify func(Action, *Container)) (chan<- bool, error) {
	return c.WatchContainerMatching(func(Action, *Container) bool { return true }, notify)
}

func (c *Docker) WatchContainerMatching(accept func(Action, *Container) bool, notify func(Action, *Container)) (chan<- bool, error) {
	stop := make(chan bool, 1)
	events := make(chan *_docker.APIEvents)
	err := c.docker.AddEventListener(events)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case event := <-events:
				glog.V(100).Infoln("Docker event:", event)

				action, has := verbs[event.Status]
				if !has {
					continue
				}

				container := &Container{Id: event.ID, Image: event.From, docker: c.docker}
				if action != Remove {
					err := container.Inspect()
					if err != nil {
						glog.Warningln("Error inspecting container", event.ID)
						continue
					}
				}

				if watch != nil && accept(action, container) {
					notify(action, container)
				}

			case done := <-stop:
				if done {
					glog.Infoln("Watch terminated.")
					return
				}
			}
		}
	}()
	return stop, nil
}

func (c *Container) Inspect() error {
	cc, err := c.docker.InspectContainer(c.Id)
	if err != nil {
		return err
	}
	c.Name = cc.Name[1:] // there's this funny '/name' thing going on with how docker names containers
	c.ImageId = cc.Image
	c.Command = cc.Path + " " + strings.Join(cc.Args, " ")
	if cc.NetworkSettings != nil {
		c.Ip = cc.NetworkSettings.IPAddress
		c.Network = *cc.NetworkSettings
		c.Ports = get_ports(cc.NetworkSettings.PortMappingAPI())
	}
	c.DockerData = cc
	return nil
}

func get_ports(list []_docker.APIPort) []Port {
	out := make([]Port, len(list))
	for i, p := range list {
		out[i] = Port{
			ContainerPort: p.PrivatePort,
			HostPort:      p.PublicPort,
			Type:          p.Type,
			AcceptIP:      p.IP,
		}
	}
	return out
}

// Note that this depends on the context in which it is run.
// If this is run from the host (outside container), then it will return the address at eth0,
// but if it's run from inside a container, the eth0 interface is actually the docker0 interface
// on the host.
func GetEth0Ip() ([]string, error) {
	ips := []string{}
	intf, err := net.InterfaceByName("eth0")
	if err != nil {
		return ips, err
	}

	addrs, err := intf.Addrs()
	if err != nil {
		return ips, err
	}

	for _, a := range addrs {
		// parse the ip in CIDR form
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			return nil, err
		}
		ips = append(ips, ip.String())
	}
	return ips, nil
}
