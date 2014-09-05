package main

import (
	"fmt"
	"github.com/qorio/maestro/util"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"log"
	"os"
	"text/template"
)

var _data = `
a: Easy!
b: #comment here.

  c: 2
  d: [3, 4]
  e:
    f:
     k1: v1
     k2: v2
`
var data = `
a: Easy!
b: #comment here.
  - c : &c1
     cc : cc2
  - c : &c2
     cc : cc3
bb:
  c: 2
  d: [3, 4]
  e:
    f:
     k1: v1
     k2: v2
kk:
  - *c2
  - *c1
`

type T struct {
	A string
	B struct {
		C int
		D []int ",flow"
	}
}

func main() {

	if len(os.Args) == 1 {
		s := util.ServiceSpec{
			CircleCI:    "circle",
			BuildNumber: 20,
		}

		fmt.Println("spec=%s", s)

		t := T{}

		err := yaml.Unmarshal([]byte(data), &t)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("--- t:\n%v\n\n", t)

		d, err := yaml.Marshal(&t)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("--- t dump:\n%s\n\n", string(d))

		m := make(map[string]interface{})

		err = yaml.Unmarshal([]byte(data), &m)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("--- m:\n%v\n\n", m)

		d, err = yaml.Marshal(&m)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		fmt.Printf("--- m dump:\n%s\n\n", string(d))
	}
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
