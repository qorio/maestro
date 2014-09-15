package ssh

import (
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.crypto/ssh/agent"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"strings"
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

type PublicKey struct {
	Algo  string
	Bytes []byte
	User  string
	Host  string
}

func ParsePublicKeyFile(keyfile string) (*PublicKey, error) {
	keybytes, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}
	return ParsePublicKey(keybytes)
}

func ParsePublicKey(keyBytes []byte) (*PublicKey, error) {
	parts := strings.Split(string(keyBytes), " ")
	if len(parts) != 3 {
		return nil, errors.New("Bad format")
	}

	contact := strings.Split(strings.Trim(parts[2], " "), "@")
	return &PublicKey{
		Algo:  strings.Trim(parts[0], " "),
		Bytes: []byte(parts[1]),
		User:  contact[0],
		Host:  contact[1],
	}, nil
}

func KeyFileAuthMethod(keyfile string) (ssh.AuthMethod, error) {
	key, err := ParseKey(keyfile)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(key), nil
}

func AgentAuthMethod() (ssh.AuthMethod, error) {
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

func KeyBytesAuthMethod(buff []byte) (ssh.AuthMethod, error) {
	private, err := ssh.ParsePrivateKey(buff)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(private), nil
}

func NewClient(user, host string, auth ssh.AuthMethod) (*sshClient, error) {
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
