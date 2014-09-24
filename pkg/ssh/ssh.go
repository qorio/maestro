package ssh

import (
	"bufio"
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.crypto/ssh/agent"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
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

const (
	SCP_PUSH_BEGIN_FILE = "C"
	SCP_PUSH_END        = "\x00"
)

func (this *sshClient) CopyFile(srcFile, destFile string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	stat, err := src.Stat()
	if err != nil {
		return err
	}
	return this.Copy(src, stat.Mode(), stat.Size(), destFile)
}

func (this *sshClient) CopyBytes(data []byte, perm os.FileMode, destFile string) error {
	src := bytes.NewBuffer(data)
	return this.Copy(src, perm, int64(src.Len()), destFile)
}

func (this *sshClient) Copy(data io.Reader, perm os.FileMode, size int64, destFile string) error {
	session, err := this.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	go func() {
		r, _ := session.StdoutPipe()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			glog.Infoln("scp-remote:", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			glog.Warningln("Err", err)
		}
	}()
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		perm := fmt.Sprintf("%#o", uint32(perm))
		// According to https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works
		// Print the file content
		glog.Infoln("scp-local:", SCP_PUSH_BEGIN_FILE+perm, size, filepath.Base(destFile))

		fmt.Fprintln(w, SCP_PUSH_BEGIN_FILE+perm, size, filepath.Base(destFile))
		io.Copy(w, data)
		fmt.Fprint(w, SCP_PUSH_END)

		glog.Infoln("scp-local: completed")
	}()
	glog.Infoln("scp-local: Staring with /usr/bin/scp -qrt " + filepath.Dir(destFile))
	err = session.Run("/usr/bin/scp -t " + filepath.Dir(destFile))
	if err != nil {
		glog.Infoln("Error", err)
		return err
	}

	return nil
}
