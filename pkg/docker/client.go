package docker

import (
	_docker "github.com/fsouza/go-dockerclient"
	"net"
	"strings"
)

type client struct {
	endpoint string
	docker   *_docker.Client
}

type Container struct {
	Id          string
	Ip          string // the ip from the private network over docker0 interface.
	Image       string
	Command     string
	PortBinding map[_docker.Port][]_docker.PortBinding
}

func NewClient(endpoint string) (c *client, err error) {
	c = &client{endpoint: endpoint}
	c.docker, err = _docker.NewClient(endpoint)
	return c, err
}

func (c *client) GetContainer(id string) (*Container, error) {
	cc, err := c.docker.InspectContainer(id)
	if err != nil {
		return nil, err
	}

	return &Container{
		Id:          cc.ID,
		Ip:          cc.NetworkSettings.IPAddress,
		Image:       cc.Image,
		Command:     cc.Path + " " + strings.Join(cc.Args, " "),
		PortBinding: cc.NetworkSettings.Ports,
	}, nil
}

func (c *client) ListContainers() ([]*Container, error) {
	l, err := c.docker.ListContainers(_docker.ListContainersOptions{
		All:  true,
		Size: true,
	})
	if err != nil {
		return nil, err
	}
	out := []*Container{}
	for _, cc := range l {
		out = append(out, &Container{
			Id:      cc.ID,
			Image:   cc.Image,
			Command: cc.Command,
		})
	}
	return out, nil
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
