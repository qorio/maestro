package util

import (
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.crypto/ssh/agent"
	"errors"
	"io/ioutil"
	"net"
	"os"
)

func ParseKey(file string) (ssh.Signer, error) {
	privateKeyBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	private, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	return private, nil
}

type sshClient struct {
	config *ssh.ClientConfig
	client *ssh.Client
}

func SshKeyFileAuthMethod(keyfile string) (ssh.AuthMethod, error) {
	key, err := ParseKey(keyfile)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(key), nil
}

func SshAgentAuthMethod() (ssh.AuthMethod, error) {
	env := os.Getenv("SSH_AUTH_SOCK")
	if env == "" {
		return nil, errors.New("SSH_AUTH_SOCK not defined")
	}
	conn, err := net.Dial("unix", env)
	if err != nil {
		return nil, err
	}
	agent := agent.NewClient(conn)
	auths, err := agent.Signers()
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(auths...), nil
}

func SshKeyBytesAuthMethod(buff []byte) (ssh.AuthMethod, error) {
	private, err := ssh.ParsePrivateKey(buff)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(private), nil
}

func NewSshClient(user, host string, auth ssh.AuthMethod) (*sshClient, error) {
	c := &sshClient{
		config: &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				auth,
			},
		},
	}
	client, err := ssh.Dial("tcp", host+":22", c.config)
	if err != nil {
		return nil, err
	}
	c.client = client
	return c, nil
}

func (this *sshClient) RunCommandStdout(cmd string) ([]byte, error) {
	session, err := this.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(cmd)
}
