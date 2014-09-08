package util

import (
	"bufio"
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	_ "crypto"
	_ "crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"os"
	"testing"
)

func TestSsh(t *testing.T) { TestingT(t) }

type SSHTests struct{}

var _ = Suite(&SSHTests{})

func parseKey(file string) (ssh.Signer, error) {
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

func parseKey2(file string) (ssh.Signer, error) {
	privateKeyBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(privateKeyBytes)
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func command0(session *ssh.Session, s string) (string, error) {
	ok, err := session.SendRequest("command", true, []byte(s))
	if !ok || err != nil {
		return "", err
	}
	n, _ := session.StdoutPipe()
	session.Wait()

	r := bufio.NewReader(n)
	i, _, _ := r.ReadLine()
	return string(i), nil
}

func command(session *ssh.Session, s string) (string, error) {
	var stdout bytes.Buffer
	session.Stdout = &stdout
	err := session.Run(s)
	if err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func run(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(cmd)
}

func (suite *SSHTests) TestSshExecute(c *C) {

	// Note -- golang's ssh private rsa key parser doesn't support AES-128-CBC cipher
	// Create the ssh key just by ssh-keygen -t rsa -C <email>

	//keyfile := os.Getenv("HOME") + "/SparkleShare/documents/self/.ssh/github_rsa-qorio"
	//keyfile := os.Getenv("HOME") + "/SparkleShare/documents/self/.ssh/google_compute_engine"
	keyfile := os.Getenv("HOME") + "/.ssh/id_rsa"
	privateKey, err := parseKey(keyfile)
	c.Assert(err, Equals, nil)

	config := &ssh.ClientConfig{
		User: "ops",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
	}

	client, err := ssh.Dial("tcp", "146.148.63.108:22", config)
	c.Assert(err, Equals, nil)

	{
		stdout, err := run(client, "docker ps")
		c.Log(string(stdout), err)
	}
	{
		stdout, err := run(client, "ls -al")
		c.Log(string(stdout), err)
	}
	{
		stdout, err := run(client, "ps -ef")
		c.Log(string(stdout), err)
	}
}
