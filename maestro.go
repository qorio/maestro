package main

import (
	"fmt"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"log"
	"os"
	"text/template"
)

func main() {

	// Try the test file
	file, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}

	buff, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	cc := make(map[interface{}]interface{})
	err = yaml.Unmarshal(buff, &cc)
	if err != nil {
		panic(err)
	}
	fmt.Println("cc=%s", cc)
	x, err := yaml.Marshal(&cc)
	if err != nil {
		panic(err)
	}
	fmt.Println("Read:\n", string(x))

	const test = "Hello this is {{.name}} and mounted on {{.db.mount}}"
	c := map[string]interface{}{
		"name": "foo",
		"db":   map[string]string{"mount": "/mnt/v1"},
	}

	t2 := template.Must(template.New("letter2").Parse(test))
	err = t2.Execute(os.Stdout, c)
	if err != nil {
		log.Println("executing template:", err)
	}
}
