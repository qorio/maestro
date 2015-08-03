package template

import (
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	ErrNotSupportedProtocol = errors.New("protocol-not-supported")
	ErrNotConnectedToZk     = errors.New("not-connected-to-zk")
)

func ApplyTemplate(body string, context interface{}, funcs ...template.FuncMap) (string, error) {
	var t *template.Template
	var err error

	if len(funcs) > 0 {
		t, err = template.New(body).Funcs(funcs[0]).Parse(body)
		if err != nil {
			return "", err
		}
	} else {
		t, err = template.New(body).Parse(body)
		if err != nil {
			return "", err
		}
	}

	var buff bytes.Buffer
	if err := t.Execute(&buff, context); err != nil {
		return "", err
	} else {
		return buff.String(), nil
	}
}

func ParseHostPort(value string) (host, port string) {
	parts := strings.Split(value, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func FetchUrl(urlRef string, headers map[string]string, zc ...zk.ZK) (body string, mime string, err error) {
	switch {
	case strings.Index(urlRef, "http://") == 0, strings.Index(urlRef, "https://") == 0:
		url, err := url.Parse(urlRef)
		if err != nil {
			return "", "", err
		}

		// don't check certificate for https
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		req, err := http.NewRequest("GET", url.String(), nil)

		for h, v := range headers {
			req.Header.Add(h, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}
		content, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", "", err
		}
		return string(content), resp.Header.Get("Content-Type"), nil

	case strings.Index(urlRef, "file://") == 0:
		file := urlRef[len("file://"):]
		f, err := os.Open(file)
		if err != nil {
			return "", "", err
		}
		defer f.Close()
		if buff, err := ioutil.ReadAll(f); err != nil {
			return "", "", err
		} else {
			return string(buff), "text/plain", nil
		}

	case strings.Index(urlRef, "string://") == 0:
		content := urlRef[len("string://"):]
		return content, "text/plain", nil

	case strings.Index(urlRef, "env://") == 0:
		if len(zc) == 0 {
			return "", "", ErrNotConnectedToZk
		}
		path := urlRef[len("env://"):]
		n, err := zc[0].Get(path)
		if err != nil {
			return "", "", err
		}
		glog.Infoln("Content from environment: Path=", urlRef, "Err=", err)
		// try resolve
		_, v, err := zk.Resolve(zc[0], registry.Path(path), n.GetValueString())
		if err != nil {
			return "", "", err
		}
		return v, "text/plain", nil
	}
	return "", "", ErrNotSupportedProtocol
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

func ExecuteTemplateUrl(zc zk.ZK, url string, authToken string, data interface{}, funcs ...template.FuncMap) ([]byte, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + authToken,
	}

	config_template_text, _, err := FetchUrl(url, headers, zc)
	if err != nil {
		glog.Warningln("Error fetching template:", err)
		return nil, err
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
		"inline": func(url string) (string, error) {
			// We support variables inside the function argument
			u, err := apply_template(url, url, data)
			if err != nil {
				return "", err
			}
			content, _, err := FetchUrl(string(u), headers, zc)
			if err != nil {
				return "", err
			}
			return content, nil
		},
		"file": func(url string, dir ...string) (string, error) {
			// We support variables inside the function argument
			u, err := apply_template(url, url, data)
			if err != nil {
				return "", err
			}
			content, _, err := FetchUrl(string(u), headers, zc)
			if err != nil {
				return "", err
			}
			// Write to local file and return the path
			parent := os.TempDir()
			if len(dir) > 0 {
				parent = dir[0]
				// We support variables inside the function argument
				p, err := apply_template(parent, parent, data)
				if err != nil {
					return "", err
				}
				parent = string(p)
			}
			path := filepath.Join(parent, filepath.Base(string(u)))
			err = ioutil.WriteFile(path, []byte(content), 0777)
			glog.Infoln("Written", len([]byte(content)), " bytes to", path, "Err=", err)
			if err != nil {
				return "", err
			}
			return path, nil
		},
	}

	if len(funcs) > 0 {
		for k, f := range funcs[0] {
			funcMap[k] = f
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
