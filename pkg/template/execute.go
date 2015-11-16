package template

import (
	"bytes"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func ApplyTemplate(body string, context interface{}, funcs ...template.FuncMap) (string, error) {
	content, err := apply_template(body, body, context, funcs...)
	return string(content), err
}

func ExecuteUrl(zc zk.ZK, url string, authToken string, data interface{}, funcs ...template.FuncMap) ([]byte, error) {
	headers := map[string]string{}
	if len(authToken) > 0 {
		headers["Authorization"] = "Bearer " + authToken
	}

	fetch_with_headers := func(url string) (string, string, error) {
		// We support variables inside the function argument
		u, err := apply_template(url, url, data)
		applied_url := string(u)
		if err != nil {
			return "", applied_url, err
		}
		content, _, err := FetchUrl(applied_url, headers, zc)
		if err != nil {
			return "", applied_url, err
		}
		return content, applied_url, nil
	}

	var config_template_text string
	var err error
	switch {
	case strings.Index(url, "func://") == 0 && len(funcs) == 1:
		if f, has := funcs[0][url[len("func://"):]]; has {
			if ff, ok := f.(func() string); ok {
				config_template_text = ff()
			} else {
				glog.Warningln("Bad function:", url)
				return nil, ErrBadTemplateFunc
			}
		} else {
			glog.Warningln("Error no function:", url)
			return nil, ErrMissingTemplateFunc
		}
	default:
		config_template_text, _, err = FetchUrl(url, headers, zc)
		if err != nil {
			glog.Warningln("Error fetching template:", err)
			return nil, err
		}
	}

	funcMap := template.FuncMap{
		"containers": func(path, service_port string) ([]interface{}, error) {
			// We support variables inside the function argument
			p, err := apply_template(path, path, data)
			if err != nil {
				return nil, err
			}
			return hostport_list_from_zk(zc, string(p), service_port)
		},
		"host": func() int {
			return 0
		},
		"port": func() int {
			return 1
		},
		"hostport": func(hostport string, p int) string {
			host, port := ParseHostPort(hostport)
			switch p {
			case 0:
				return host
			case 1:
				return port
			}
			return ""
		},
		"members": func(path string) ([]*zk.Node, error) {
			glog.Infoln("Path=", path)
			// We support variables inside the function argument
			parent, err := apply_template(path, path, data)
			if err != nil {
				return nil, err
			}
			n, err := zk.Follow(zc, registry.Path(parent))
			if err != nil {
				return nil, err
			}
			return n.Children()
		},
		"inline": func(url string) (string, error) {
			content, _, err := fetch_with_headers(url)
			return content, err
		},
		"shell": func(line string) error {
			return ExecuteShell(line)
		},
		"file": func(url string, opts ...string) (string, error) {
			content, applied_url, err := fetch_with_headers(url)
			if err != nil {
				return "", err
			}
			// Write to local file and return the path, unless the
			// path is provided.
			parent := os.TempDir()
			if len(opts) >= 1 {
				parent = opts[0]
				// We support variables inside the function argument
				p, err := apply_template(parent, parent, data)
				if err != nil {
					return "", err
				}
				parent = string(p)
			}
			// Default permission unless it's provided
			var perm os.FileMode = 0777
			if len(opts) >= 2 {
				permString := opts[1]
				perm = FileModeFromString(permString)
			}
			// path can be either a filepath or a directory
			// check the path to see if it's a directory
			fpath := parent
			fi, err := os.Stat(parent)
			if os.IsNotExist(err) || !fi.IsDir() {
				// use the path as is
				fpath = parent
			} else if fi.IsDir() {
				// build the name
				fpath = filepath.Join(parent, filepath.Base(string(applied_url)))
			}
			err = ioutil.WriteFile(fpath, []byte(content), perm)
			glog.Infoln("Written", len([]byte(content)), " bytes to", fpath, "perm=", perm.String(), "Err=", err)
			if err != nil {
				return "", err
			}
			return fpath, nil
		},
	}

	// Merge the input func map with the predefined ones.
	// Note input functions cannot re-define existing ones.
	if len(funcs) > 0 {
		for k, f := range funcs[0] {
			if _, has := funcMap[k]; !has {
				funcMap[k] = f
			}
		}
	}

	config_template, err := template.New(url).Funcs(funcMap).Parse(config_template_text)
	if err != nil {
		glog.Warningln("Error parsing template", url, err)
		return nil, err
	}
	var buff bytes.Buffer
	err = config_template.Execute(&buff, data)
	return buff.Bytes(), err
}
