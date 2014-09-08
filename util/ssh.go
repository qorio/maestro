package util

import (
	"code.google.com/p/go.crypto/ssh"
	"io/ioutil"
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

func ExecCommand(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(cmd)
}
