package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/zk"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Value set by ldflag (-X main.BUILD_VERSION version) during build
var (
	BUILD_VERSION   string
	BUILD_TIMESTAMP string
)
var (
	stdin   = flag.Bool("stdin", false, "User stdin for input")
	stdout  = flag.Bool("stdout", false, "Wrtie to stdout")
	hosts   = flag.String("hosts", "localhost:2181", "Comma-delimited zk host:port")
	timeout = flag.Duration("timeout", time.Second, "Connection timeout to zk.")
	root    = flag.String("root", "", "Root key to scan the children znodes")
	escape  = flag.Bool("escape", false, "Escape space")
	newline = flag.Bool("newline", false, "New line")
	quote   = flag.String("quote", "", "Quote character")
	logto   = flag.String("logto", "stdout", "Command logs to stdout | stderr | tee")
)

func read_from_stdin() ([]string, map[string]string) {
	keys := make([]string, 0)
	env := make(map[string]string)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		i := strings.Index(line, "=")
		if i > 0 {
			key := line[0:i]
			value := line[i+1:]
			keys = append(keys, key)
			env[key] = value
		}
	}
	sort.Strings(keys)
	return keys, env
}

func read_from_zk() ([]string, map[string]string) {
	zookeeper, err := zk.Connect(strings.Split(*hosts, ","), *timeout)
	if err != nil {
		panic(err)
	}
	defer zookeeper.Close()

	root_node, err := zookeeper.Get(*root)
	if err != nil {
		panic(err)
	}

	// Just get the entire set of values and export them as environment variables
	all, err := root_node.FilterChildrenRecursive(func(z *zk.Node) bool {
		return !z.IsLeaf() // filter out parent nodes
	})

	if err != nil {
		panic(err)
	}

	keys := make([]string, 0)
	env := make(map[string]string)
	for _, node := range all {
		key := node.GetBasename()
		value := node.GetValueString()
		env[key] = value
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys, env
}

// Assume there's a variable in zk under /zk/root/VAR, this will export VAR as environment variable.
// export $(go run main/zk_env.go -hosts="localhost:21810" -root=/zk/root | xargs) && echo $VAR
// http://stackoverflow.com/questions/19331497/set-environment-variables-from-file
func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s version %s, built on %s\n", os.Args[0], BUILD_VERSION, BUILD_TIMESTAMP)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	keys := make([]string, 0)
	env := make(map[string]string)
	if *stdin {
		glog.Infoln("Environment from stdin:")
		keys, env = read_from_stdin()
	} else {
		glog.Infoln("Environment from zookeeper:", *hosts)
		keys, env = read_from_zk()
	}

	for _, k := range keys {
		os.Setenv(k, env[k])
		value := env[k]
		switch {
		case strings.ContainsAny(value, " \t\n"):
			if *escape {
				value = strings.Replace(value, " ", "\\ ", -1)
			}
			value = *quote + value + *quote
		case value == "":
			value = *quote + *quote
		default:
			value = env[k]
		}

		format := "%s=%s"
		if *newline {
			format = format + "\n"
		} else {
			format = format + " "
		}
		if *stdout {
			fmt.Fprintf(os.Stdout, format, k, value)
		}
		glog.Infoln(fmt.Sprintf("%s=%s", k, value))
	}

	if len(flag.Args()) > 0 {
		command := flag.Args()[0]
		args := make([]string, 0)
		if len(flag.Args()[1:]) > 0 {
			args = flag.Args()[1:]
		}
		glog.Infoln("Starting", flag.Args())
		cmd := exec.Command(command, args...)

		output := make(chan interface{})

		switch *logto {
		case "stdout":
			proc_stdout, err := cmd.StdoutPipe()
			if err != nil {
				panic(err)
			}
			go func() {
				sc := bufio.NewScanner(proc_stdout)
				for sc.Scan() {
					output <- sc.Text()
				}
				output <- nil
			}()
		case "stderr":
			proc_stderr, err := cmd.StderrPipe()
			if err != nil {
				panic(err)
			}
			go func() {
				sc := bufio.NewScanner(proc_stderr)
				for sc.Scan() {
					output <- sc.Text()
				}
				output <- nil
			}()
		case "tee":
			proc_stdout, err := cmd.StdoutPipe()
			if err != nil {
				panic(err)
			}
			go func() {
				sc := bufio.NewScanner(proc_stdout)
				for sc.Scan() {
					output <- sc.Text()
				}
				output <- nil
			}()
			proc_stderr, err := cmd.StderrPipe()
			if err != nil {
				panic(err)
			}
			go func() {
				sc := bufio.NewScanner(proc_stderr)
				for sc.Scan() {
					output <- sc.Text()
				}
				output <- nil
			}()
		}

		cmd.Start()
		count := 1
		if *logto == "tee" {
			count = 2
		}

		for count > 0 {
			line := <-output
			if line == nil {
				count = count - 1
			} else {
				fmt.Println(line)
			}
		}
	}
}
