package docker

import (
	"bufio"
	"bytes"
	_docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"io"
	"net"
	"strings"
)

type Docker struct {
	Endpoint string

	Cert string
	Key  string
	Ca   string

	docker *_docker.Client
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

type ContainerControl struct {
	ContainerName string `json:"name"`
	*_docker.Config

	HostConfig *_docker.HostConfig `json:"host_config"`
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
		out = append(out, &Container{
			Id:      cc.ID,
			Image:   cc.Image,
			Command: cc.Command,
			Ports:   get_ports(cc.Ports),
			docker:  c.docker,
		})
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

func (c *Docker) StartDaemon(auth *AuthIdentity, ct *ContainerControl) (*Container, error) {
	return c.StartContainer(auth, ct, true)
}

func (c *Docker) StartContainer(auth *AuthIdentity, ct *ContainerControl, daemon bool) (*Container, error) {
	opts := _docker.CreateContainerOptions{
		Name:       ct.ContainerName,
		Config:     ct.Config,
		HostConfig: ct.HostConfig,
	}

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
	err = c.docker.StartContainer(cc.ID, ct.HostConfig)
	if err != nil {
		return nil, err
	}

	container := &Container{
		Id:     cc.ID,
		Image:  ct.Image,
		docker: c.docker,
	}

	err = container.Inspect()
	return container, err
}

type Action int

const (
	Start Action = iota
	Stop
	Remove
)

// Docker event status are create -> start -> die -> stop for a container then destroy for docker -rm
var verbs map[string]Action = map[string]Action{
	"start":   Start,
	"stop":    Stop,
	"destroy": Remove,
}

func (c *Docker) WatchContainer(notify func(Action, *Container)) (chan<- bool, error) {
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

				if watch != nil {
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
