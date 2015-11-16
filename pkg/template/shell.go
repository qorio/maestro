package template

import (
	"github.com/golang/glog"
	"io"
	"os"
	"os/exec"
)

func ExecuteShell(line string) error {
	c := exec.Command("sh", "-")
	if stdout, err := c.StdoutPipe(); err == nil {
		go func() {
			io.Copy(os.Stdout, stdout)
		}()
	} else {
		return err
	}

	if stderr, err := c.StderrPipe(); err == nil {
		go func() {
			io.Copy(os.Stderr, stderr)
		}()
	} else {
		return err
	}

	stdin, err := c.StdinPipe()
	if err != nil {
		return err
	}

	if err := c.Start(); err != nil {
		return err
	}
	glog.Infoln("Start:", line)

	if _, err := stdin.Write([]byte(line)); err != nil {
		stdin.Close()
		return err
	}
	stdin.Close() // finished
	err = c.Wait()
	glog.Infoln("Process wait completed:", err)
	if ee, ok := err.(*exec.ExitError); ok {
		glog.Infoln("PID", ee.Pid(), " - Process state", ee.Success())
	}
	return err
}
