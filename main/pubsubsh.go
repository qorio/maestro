package main

import (
	"bufio"
	"flag"
	"fmt"
	_ "github.com/qorio/maestro/pkg/mqtt"
	. "github.com/qorio/maestro/pkg/pubsub"
	"github.com/qorio/omni/common"
	"os"
)

// Value set by ldflag (-X main.BUILD_VERSION version) during build
var (
	BUILD_VERSION   string
	BUILD_TIMESTAMP string
)
var (
	topic = flag.String("topic", "", "Topic")
)

func must_not(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s version %s, built on %s\n", os.Args[0], BUILD_VERSION, BUILD_TIMESTAMP)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	id := common.NewUUID().String()

	rootTopic := Topic(*topic)
	stdinTopic := rootTopic.Sub("stdin")   // stdin for the remote
	stdoutTopic := rootTopic.Sub("stdout") // stdout for the remote
	stderrTopic := rootTopic.Sub("stderr") // stderr for the remote

	pubsub, err := rootTopic.Broker().PubSub(id)
	must_not(err)

	stdout, err := pubsub.Subscribe(stdoutTopic)
	must_not(err)

	stderr, err := pubsub.Subscribe(stderrTopic)
	must_not(err)

	stdin := make(chan []byte)
	go func() {
		for {
			select {
			case m := <-stdout:
				fmt.Print(string(m))
				fmt.Print("dash% ")
			case m := <-stderr:
				fmt.Print(string(m))
				fmt.Print("dash% ")
			case <-stdin:
			}
		}
	}()

	in := GetWriter(stdinTopic, pubsub)
	keys := bufio.NewScanner(os.Stdin)
	fmt.Print("dash% ")
	for keys.Scan() {
		line := []byte(keys.Text() + "\n")
		in.Write(line)
		stdin <- line
	}
}
