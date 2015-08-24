package docker

import (
	. "gopkg.in/check.v1"
	"net"
	"testing"
)

func TestDockerClient(t *testing.T) { TestingT(t) }

type DockerClientTests struct{}

var _ = Suite(&DockerClientTests{})

const dockerEndpoint = "unix:///var/run/docker.sock"

func (suite *DockerClientTests) TestConnectDocker(c *C) {
	d, err := NewClient(dockerEndpoint)
	c.Assert(err, Equals, nil)
	c.Log("Connected", d)
	l, err := d.ListContainers()
	c.Assert(err, Equals, nil)
	for _, cc := range l {
		c.Log("container", cc.Id, "image", cc.Image)
	}

	err = l[0].Inspect()
	c.Assert(err, Equals, nil)

	addrs, err := net.InterfaceAddrs()
	c.Assert(err, Equals, nil)
	for _, a := range addrs {
		c.Log("network", a.Network(), "addr", a.String())
	}

	intf, err := net.InterfaceByName("eth0")
	c.Assert(err, Equals, nil)
	c.Log("interface", intf.Name)

	addrs, err = intf.Addrs()
	c.Assert(err, Equals, nil)
	for _, a := range addrs {
		// parse the ip in CIDR form
		ip, _, err := net.ParseCIDR(a.String())
		c.Assert(err, Equals, nil)
		c.Log("network", a.Network(), "addr", ip.String())
	}
}

// From http://docs.docker.com/v1.7/reference/api/hub_registry_spec/
func (suite *DockerClientTests) TestImageParsing(c *C) {
	c.Assert(ParseImageUrl("https://<registry>/repositories/samalba/busybox"), DeepEquals, Image{
		Registry:   "https://<registry>/repositories",
		Repository: "samalba/busybox",
	})
	c.Assert(ParseImageUrl("https://<registry>/repositories/samalba/busybox:12"), DeepEquals, Image{
		Registry:   "https://<registry>/repositories",
		Repository: "samalba/busybox",
		Tag:        "12",
	})
	c.Assert(ParseImageUrl("http://host.com:8080/repositories/samalba/busybox:latest"), DeepEquals, Image{
		Registry:   "http://host.com:8080/repositories",
		Repository: "samalba/busybox",
		Tag:        "latest",
	})

}
