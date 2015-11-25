package template

import (
	"crypto/tls"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

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

		switch {
		case strings.Index(file, "~") > -1:
			// expand tilda
			file = strings.Replace(file, "~", os.Getenv("HOME"), 1)
		case strings.Index(file, "./") > -1:
			// expand tilda
			file = strings.Replace(file, "./", os.Getenv("PWD")+"/", 1)
		}

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

	case strings.Index(urlRef, "zk://") == 0:
		path := urlRef[len("zk://"):]
		if len(zc) == 0 {
			return "", "", ErrNotConnectedToZk
		}
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
	case strings.Index(urlRef, "env://") == 0:
		path := urlRef[len("env://"):]
		if len(zc) == 0 {
			return "", "", ErrNotConnectedToZk
		}
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
