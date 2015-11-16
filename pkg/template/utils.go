package template

import (
	"bytes"
	"fmt"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	"os"
	"strings"
	"text/template"
)

func FileModeFromString(perm string) os.FileMode {
	if len(perm) < 4 {
		perm = fmt.Sprintf("%04v", perm)
	}
	fm := new(os.FileMode)
	fmt.Sscanf(perm, "%v", fm)
	return *fm
}

func ParseHostPort(value string) (host, port string) {
	parts := strings.Split(value, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func apply_template(key, tmpl string, data interface{}, funcMap ...template.FuncMap) ([]byte, error) {
	t := template.New(key)
	if len(funcMap) > 0 {
		t = t.Funcs(funcMap[0])
	}
	t, err := t.Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var buff bytes.Buffer
	err = t.Execute(&buff, data)
	return buff.Bytes(), err
}

// Supports references -- if the value of the node is env:///.. then resolve the reference.
func hostport_list_from_zk(zc zk.ZK, containers_path, service_port string) ([]interface{}, error) {
	n, err := zk.Follow(zc, registry.Path(containers_path))
	if err != nil {
		return nil, err
	}

	all, err := n.VisitChildrenRecursive(func(z *zk.Node) bool {
		_, port := ParseHostPort(z.GetBasename())
		return port == service_port && z.IsLeaf()
	})
	if err != nil {
		return nil, err
	}

	list := make([]interface{}, 0)
	for _, c := range all {
		host, port := ParseHostPort(c.GetValueString())
		list = append(list, struct {
			Host string
			Port string
		}{
			Host: host,
			Port: port,
		})
	}
	return list, nil
}
