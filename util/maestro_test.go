package util

import (
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v1"
	"testing"
)

const test_yml = `
global:
  CIRCLECI_API_TOKEN: b71701145614b93a382a8e3b5d633ee71c360315c
  DOCKER_ACCOUNT: lab616
  DOCKER_EMAIL: davidc616@gmail.com
  DOCKER_AUTH: bGFiNjE2OmxhYjYxNgc==

build:
 passport:
     project: qorio/omni
     build_number: 292
     artifact: passport
     dockerfile: docker/passport/Dockerfile
     image: qorio/passport:0.1

 shorty:
     project: qorio/omni
     build_number: 292
     artifact: shorty
     dockerfile: docker/shorty/Dockerfile
     image: qorio/shorty:0.1

# docker objects are bound with build and resource objects
docker:
  passport:
      ssh:
         - docker run -d -p  -t # steps

  shorty:
      ssh:
        - docker run -p -t # steps

  mongodb:
      image: mongo:2.7.5
      ssh:
        - docker run -d -p 27017:27017 -v {{.resource.disk.mongo-db.mount}}:/data/db --name mongodb {{.image}}

  redis:
      image: redis:2.8.13
      ssh:
        - docker run -d -p 6379:6379 -v {{.resource.disk.redisdb-stage.mount}}:/data
          -v /some/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  redis-prod:
      image: redis:2.8.13
      ssh:
        - >
          docker run -d -p 6379:6379 -v {{.resource.disk.redisdb-prod.mount}}:/data
           -v /some/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  nginx:
      image: nginx:1.7.1
      ssh:
        - docker run -d -v /some/nginx.conf:/etc/nginx.conf:ro --name nginx {{.image}}


service:

  passport:
      - monogdb
      - passport

  passport-prod:
      - monogdb-prod
      - passport
      - nginx


resource:

  host:
    gce-host-0:
      cloud: gce
      ip: 127:0:1:1

    gce-host-1:
      cloud: gce
      ip: 127:0:0:1

  disk:

    mongodb-prod:
      name: gce-ssd-1
      host: gce-host-1
      mount: /data

    mongodb-stage:
      name: gce-ssd-0
      host: gce-host-0
      mount: /data

    redisdb-prod:
      name: gce-ssd-1
      host: gce-host-1
      mount: /data

    redisdb-stage:
      name: gce-ssd-0
      host: gce-host-0
      mount: /data
`

func Test(t *testing.T) { TestingT(t) }

type suite struct{}

var _ = Suite(&suite{})

func (suite *suite) TestSampleDoc(c *C) {

	config := make(map[interface{}]interface{})
	err := yaml.Unmarshal([]byte(test_yml), &config)
	c.Assert(err, Equals, nil)
	c.Log("config", config)

	text, err := yaml.Marshal(&config)
	c.Log(string(text))
	c.Assert(err, Equals, nil, Commentf("Error marshaling"))
}
