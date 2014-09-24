package ssh

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	. "gopkg.in/check.v1"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSsh(t *testing.T) { TestingT(t) }

type SSHTests struct{}

var _ = Suite(&SSHTests{})

const rsa_public_key = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC9vYbN6KszdzszSTPoixN2TCe2QsIeVjEB7VzGLfkRtCs5uAwfaofBCCLSXiuP7TJVt9Y3RF7oKT0KYL5l+82HYfasD+9iHtqScGIsPeVyU4gVaXBp9drc2DLe4K1RuoRZToSHNKlCQnp/dZhLPQY/38B27QRX1OgXaJjslDIYSQ3toLHzik5C2FB6uqM7VakpoKkBinR6gJPRtlXyjZeY2jUii9ikFy7InXu/zlv/WHaVJVKah5A/lhcbaVEaO17jam62bk2gw1d9Ng5du/7zWgeJpFkZjvDwf25lffcDFzX32T+hkexraWtiwe3y3/0qTtd1IG1yOWcY81oXYtj/ test@qoriolabs.com`

const rsa_key = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAvb2GzeirM3c7M0kz6IsTdkwntkLCHlYxAe1cxi35EbQrObgM
H2qHwQgi0l4rj+0yVbfWN0Re6Ck9CmC+ZfvNh2H2rA/vYh7aknBiLD3lclOIFWlw
afXa3Ngy3uCtUbqEWU6EhzSpQkJ6f3WYSz0GP9/Adu0EV9ToF2iY7JQyGEkN7aCx
84pOQthQerqjO1WpKaCpAYp0eoCT0bZV8o2XmNo1IovYpBcuyJ17v85b/1h2lSVS
moeQP5YXG2lRGjte42putm5NoMNXfTYOXbv+81oHiaRZGY7w8H9uZX33Axc199k/
oZHsa2lrYsHt8t/9Kk7XdSBtcjlnGPNaF2LY/wIDAQABAoIBAAPriH90kSTtD7IN
jx780JgfFqrzUiNiUHzjtIj7piuxdzsoDEgR4WAhlOaXv3qnweR3eduGdBeD0pkG
Sqi8vRBQ+9GuOpniXU2tOLoFRVpJU4Rtuz7VL4JkjRiBt9L6cYWO07ofYmfWC8Xa
Xh0f06M6MjXCy9YstebV6GLq5OYPgyQCpAS5h9zHvLdRqZV5tbs9QoDMwU8yz9fI
+Vw/8MO46F7HVZ0M9izEPKnKQ9gNkzmvaKLdRUZ9F0bjP5oblUKOnufT1U12FfQL
x77NQNVoJC6ZvIFO6+qMA9phmD0zP2e5mcXvRkOzgQMCMWV3EcJ9Fpd+4EGkHq0i
0EHtEkECgYEA+9uEhU5LZ+v4TWnroounVavawP9AFasr+5Eh3gxo8zvVkGxHjSyc
OCr5x4b29eiNZStpS/5jLrq8RXVBNK7yVZM5x8sMhoGprHB8z+Qxa7mJ/vF3Z2vo
t5N8RZ2W9UgYt65YPSLawhtw0I/KZ0i3y5majnuW3RbS4fVmPA3NZy8CgYEAwNx0
p0hPDHj7thqRks+UAaJD4A6H/ICnlqB5BpqlxhTbB9f6Gv39MIW9JaiYUZwCCNsO
Zjo4xiY/NFjqwosz+urfE1xOR3Gmck3QWZc5uOo4GXhtZZHGgzNuGNL5CkcApc7L
+eK5ZX7ndCgy0TkBZFHF2nKkdWlSCmJrMxLJNzECgYEA+0T+6aA7Sur5RwKtu/Vo
dOiHzpTZ8sRblRgumcH30vOXFgdxOz+Oe9skaBQWvy/MIWs2GkMp4K0cuI9LBqyj
yQyhUNsbG/awuQFhBGe9hqQNMPTnE59tBfl2ul2HBh9vyZF/Jz9m0NFftDRA0tqR
w+bzc8OJt/nVWunhnXiHvLECgYEArz1hrcJxOVcQ+FXJ4olE5fsoC4WIoLHSFXa4
oXyRlpvKraTcd/xDO/y5cmdwB+9mld9dhRvwDHQiSBFnNuA/mgYiLjhYVGh7Ii98
WnujklcYJGSdmoXLx9lKd7nzWhhMCV0PUH5nkUavTodcLWnLzvjSe3xh3OGXDyKA
X4b5WHECgYAkdD87Rek2b/XTOIFLDacMU8L1oV0t+wu93Uml/6vdo+dr/EuOqz8u
jguW8+VwAv/kUCFWyXJoE9HELvjkUlETGBkPU/0VAK99eWylMbNOjR3Ow9cCbZrr
Ln2N5VFmCszDwgXpzVxvND2Wud5Omg1C2OsWesE62GOR7z1dbcDkCg==
-----END RSA PRIVATE KEY-----`

// Server is in
// https://console.developers.google.com/project/apps~elite-beacon-697/compute/instancesDetail/zones/us-central1-a/instances/test1

const gce_test_server = "146.148.41.142"
const gce_test_user = "test"

func (suite *SSHTests) TestParsingPrivateKey(c *C) {
	block, rest := pem.Decode([]byte(rsa_key))
	c.Assert(block.Type, Equals, "RSA PRIVATE KEY")
	c.Log("rest=", string(rest))

	pk, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	c.Assert(err, Equals, nil)
	c.Assert(pk, Not(Equals), nil)
}

func (suite *SSHTests) TestParsingPublicKey(c *C) {
	pk, err := ParsePublicKey([]byte(rsa_public_key))
	c.Assert(err, Equals, nil)
	c.Log("user=", pk.User)
}

func (suite *SSHTests) TestExecute(c *C) {

	auth, err := KeyBytesAuthMethod([]byte(rsa_key))
	c.Assert(err, Equals, nil)

	ssh_client, err := NewClient(gce_test_user, gce_test_server, auth)
	c.Assert(err, Equals, nil)
	{
		stdout, err := ssh_client.RunCommandStdout("ls -al /")
		c.Log(string(stdout), err)
	}
}

func (suite *SSHTests) TestAgent(c *C) {
	if os.Getenv("SKIP_SSH_AGENT_TEST") == "true" {
		return
	}

	auth, err := AgentAuthMethod()
	c.Assert(err, Equals, nil)

	ssh_client, err := NewClient(gce_test_user, gce_test_server, auth)
	c.Assert(err, Equals, nil)
	{
		stdout, err := ssh_client.RunCommandStdout("ls -al /")
		c.Log(string(stdout), err)
	}
}

func (suite *SSHTests) TestCopyFile(c *C) {
	// Test with the .bash_history file
	bash_history := filepath.Join(os.Getenv("HOME"), ".bash_history")
	dest := fmt.Sprintf("/home/test/bash_history-%d", time.Now().Unix())
	stat, err := os.Stat(bash_history)
	if err != nil {
		c.Log("Skipping test because we can't find ", bash_history)
		return
	}
	c.Log("source=", stat.Name(), "dest=", dest)

	auth, err := KeyBytesAuthMethod([]byte(rsa_key))
	c.Assert(err, Equals, nil)

	ssh_client, err := NewClient(gce_test_user, gce_test_server, auth)
	c.Assert(err, Equals, nil)
	{
		stdout, err := ssh_client.RunCommandStdout("pwd")
		c.Log(string(stdout), err)

		err = ssh_client.CopyFile(bash_history, dest)
		c.Log(err)

		stdout, err = ssh_client.RunCommandStdout("ls -al")
		c.Log(string(stdout), err)

		stdout, err = ssh_client.RunCommandStdout("rm -f " + dest)
		c.Log(string(stdout), err)

		stdout, err = ssh_client.RunCommandStdout("ls -al")
		c.Log(string(stdout), err)
	}
}

func (suite *SSHTests) TestCallDocker(c *C) {

	auth, err := KeyBytesAuthMethod([]byte(rsa_key))
	c.Assert(err, Equals, nil)

	ssh_client, err := NewClient(gce_test_user, gce_test_server, auth)
	c.Assert(err, Equals, nil)
	{
		stdout, err := ssh_client.RunCommandStdout("docker ps")
		c.Log(string(stdout), err)
	}
}

func (suite *SSHTests) TestCopyBytes(c *C) {

	data := []byte(`{"https://index.docker.io/v1/":{"auth":"cW9yaW9sYWJzOlFvcmlvMWxhYnMh","email":"docker@qoriolabs.com"}}`)

	dest := fmt.Sprintf("/home/test/dockercfg-%d", time.Now().Unix())
	auth, err := KeyBytesAuthMethod([]byte(rsa_key))
	c.Assert(err, Equals, nil)

	ssh_client, err := NewClient(gce_test_user, gce_test_server, auth)
	c.Assert(err, Equals, nil)
	{
		stdout, err := ssh_client.RunCommandStdout("ls -al")
		c.Log(string(stdout), err)

		err = ssh_client.CopyBytes(data, 0644, dest)
		c.Log(err)

		stdout, err = ssh_client.RunCommandStdout("ls -al")
		c.Log(string(stdout), err)

		stdout, err = ssh_client.RunCommandStdout("cat " + dest)
		c.Log(string(stdout), err)

		stdout, err = ssh_client.RunCommandStdout("rm -f " + dest)
		c.Log(string(stdout), err)

		stdout, err = ssh_client.RunCommandStdout("ls -al")
		c.Log(string(stdout), err)
	}
}
