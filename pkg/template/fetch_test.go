package template

import (
	"encoding/json"
	"fmt"
	"github.com/qorio/maestro/pkg/registry"
	"github.com/qorio/maestro/pkg/zk"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetch(t *testing.T) { TestingT(t) }

type TestSuiteFetch struct {
	zc zk.ZK
}

var _ = Suite(&TestSuiteFetch{})

// Database set up for circle_ci:
// psql> create role ubuntu login password 'password';
// psql> create database circle_ci with owner ubuntu encoding 'UTF8';
func (suite *TestSuiteFetch) SetUpSuite(c *C) {
	zc, err := zk.Connect([]string{"localhost:2181"}, 1*time.Second)
	c.Assert(err, Equals, nil)
	suite.zc = zc
}

func (suite *TestSuiteFetch) TearDownSuite(c *C) {
	suite.zc.Close()
}

func (suite *TestSuiteFetch) TestFetchUrl(c *C) {
	zk.CreateOrSet(suite.zc, "/unit-test/object/1", "object1")
	zk.CreateOrSet(suite.zc, "/unit-test/ref", "env:///unit-test/object/1")

	value, _, err := FetchUrl("env:///unit-test/ref", nil, suite.zc)
	c.Assert(err, Equals, nil)
	c.Assert(value, Equals, "object1")
}

func (suite *TestSuiteFetch) TestFetchAndExecuteTemplate(c *C) {

	list := make([]interface{}, 0)

	for i := 0; i < 10; i++ {
		port := 45167 + i
		hostport := struct {
			Host string
			Port string
		}{
			Host: "ip-10-31-81-235",
			Port: fmt.Sprintf("%d", port),
		}
		c.Log("instance= ", hostport)
		list = append(list, hostport)
	}

	data := make(map[string]interface{})
	data["HostPortList"] = list
	c.Log("Data= ", data)

	url := "http://qorio.github.io/public/nginx/nginx.conf"
	config, err := ExecuteTemplateUrl(nil, url, "", data)

	c.Assert(err, Equals, nil)
	c.Log("config= ", string(config))
}

func (suite *TestSuiteFetch) TestFetchAndExecuteTemplate2(c *C) {

	list := make([]interface{}, 0)

	for i := 0; i < 10; i++ {
		port := 45167 + i
		hostport := struct {
			Host string
			Port string
		}{
			Host: "ip-10-31-81-235",
			Port: fmt.Sprintf("%d", port),
		}
		c.Log("instance= ", hostport)
		list = append(list, hostport)
	}

	data := make(map[string]interface{})
	data["HostPortList"] = list
	c.Log("Data= ", data)

	// content
	content := "Some content"
	content_path := filepath.Join(os.TempDir(), "content.test")
	err := ioutil.WriteFile(content_path, []byte(content), 0777)
	c.Assert(err, Equals, nil)
	content_url := "file://" + content_path

	// Make up a template
	config := fmt.Sprintf(`{ "file" : "{{file "%s"}}", "inline":"{{inline "%s"}}" }`,
		content_url, content_url)

	c.Log("Config:", config)

	// write config to disk
	config_path := filepath.Join(os.TempDir(), "config.test")
	err = ioutil.WriteFile(config_path, []byte(config), 0777)
	c.Assert(err, Equals, nil)

	url := "file://" + config_path
	applied, err := ExecuteTemplateUrl(nil, url, "", data)

	c.Assert(err, Equals, nil)
	c.Log("config= ", string(applied))

	// parse the json and test
	parsed := map[string]string{}

	err = json.Unmarshal(applied, &parsed)
	c.Assert(err, Equals, nil)
	c.Log(parsed)

	c.Assert(parsed["inline"], Equals, content)
	// read the file
	f, err := os.Open(parsed["file"])
	c.Assert(err, Equals, nil)
	buff, err := ioutil.ReadAll(f)
	c.Assert(err, Equals, nil)
	c.Assert(string(buff), Equals, content)
}

var nginx = `
upstream backend {
  {{range containers "/{{.Domain}}/{{.Service}}/live" "3000"}}
    server {{.Host}}:{{.Port}};
  {{end}}
}

server {

       listen 443;
       server_name *.{{inline "env:///{{.Domain}}/{{.Service}}/env/DOMAIN"}};

       ssl on;
       ssl_certificate {{file "env:///code.qor.io/ssl/qor.io.cert"}};
       ssl_certificate_key {{file "env:///code.qor.io/ssl/qor.io.key"}};

       root /var/www/infradash/public;
       try_files $uri/index.html $uri @backend;

       location @backend {

             # Support for CORS
	     # OPTIONS indicates a CORS pre-flight request
	     if ($request_method = 'OPTIONS') {
	       add_header 'Access-Control-Allow-Origin' "*";
	       add_header 'Access-Control-Allow-Credentials' 'true';
	       add_header 'Access-Control-Max-Age' 1728000;
	       add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, OPTIONS, DELETE';
	       add_header 'Access-Control-Allow-Headers' 'Authorization,Content-Type,Accept,Origin,User-Agent,DNT,Cache-Control,X-Mx-ReqToken,Keep-Alive,X-Requested-With,If-Modified-Since';
	       add_header 'Content-Length' 0;
	       add_header 'Content-Type' 'text/plain charset=UTF-8';
	       return 204;
	     }
	     # non-OPTIONS indicates a normal CORS request
	     if ($request_method = 'GET') {
	       add_header 'Access-Control-Allow-Origin' "*";
	       add_header 'Access-Control-Allow-Credentials' 'true';
	     }
	     if ($request_method = 'POST') {
	       add_header 'Access-Control-Allow-Origin' "*";
	       add_header 'Access-Control-Allow-Credentials' 'true';
	     }
	     if ($request_method = 'PUT') {
	       add_header 'Access-Control-Allow-Origin' "*";
	       add_header 'Access-Control-Allow-Credentials' 'true';
	     }
	     if ($request_method = 'DELETE') {
	       add_header 'Access-Control-Allow-Origin' "*";
	       add_header 'Access-Control-Allow-Credentials' 'true';
	     }

	     add_header 'X-infradash-Nginx-Template' 'v0.1';

	     proxy_set_header Host $http_host;
	     proxy_set_header X-Real-IP $remote_addr;
	     proxy_set_header Client-IP $remote_addr;
	     proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
	     proxy_pass http://backend;
	 }

	 error_page 500 502 503 /500.html;
  	 error_page 504 /504.html;
	 client_max_body_size 1G;
	 keepalive_timeout 10;
}

`

func (suite *TestSuiteFetch) TestFetchAndExecuteTemplateNginxConf(c *C) {
	// create some nodes
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c1:3000"), "host1:8130")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c2:3000"), "host2:8131")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c3:3000"), "host3:8132")
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/v1/containers/c4:3100"), "host4:8133")

	// Reference node
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp"), "env:///test.com/testapp/v1/containers")

	// Inline node
	zk.CreateOrSet(suite.zc, registry.Path("/test.com/testapp/env/DOMAIN"), "test.com")

	c.Log("Data inserted into zookeeper")

	path := filepath.Join(os.TempDir(), "nginx.conf")
	err := ioutil.WriteFile(path, []byte(nginx), 0777)
	c.Assert(err, Equals, nil)
	content_url := "file://" + path

	applied, err := ExecuteTemplateUrl(suite.zc, content_url, "", map[string]string{
		"Domain": "test.com", "Service": "testapp",
	})

	c.Assert(err, Equals, nil)
	c.Log("config= ", string(applied))
}
