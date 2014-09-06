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

# docker objects reference the build of the same name.
# also, the host object is mapped to the scope of the docker object so host mounts are referenceable
docker:
  passport:
      ssh:
         - docker run -d -p 5050:5050 -v {{.host.mounts.config}}/omni:/static/conf --name passport {{.image}}

  shorty:
      ssh:
         - docker run -d -p 5050:5050 -v {{.host.mounts.config}}/omni:/static/conf --name shorty {{.image}}

  mongodb:
      image: mongo:2.7.5
      ssh:
        - docker run -d -p 27017:27017 -v {{.host.mounts.db}}/mongo:/data/db --name mongodb {{.image}}

  redis:
      image: redis:2.8.13
      ssh:
        - docker run -d -p 6379:6379 -v {{.host.mounts.db}}:/data
          -v {{.host.mounts.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  redis-prod:
      image: redis:2.8.13
      ssh:
        - >
          docker run -d -p 6379:6379 -v {{.resource.disk.redisdb-prod.mount}}:/data
           -v {{.host.mounts.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  nginx:
      image: nginx:1.7.1
      ssh:
        - docker run -d -v /some/nginx.conf:/etc/nginx.conf:ro --name nginx {{.image}}


service:

  passport:
      monogdb: db
      passport: dev

  passport-prod:
      - monogdb-prod: prod-db
      - passport: prod
      - nginx: lb


resource:

  disk:

    dev-configs:
      cloud: gce
      type: disk
      size: 50MB

    dev-db:
      cloud: gce
      type: disk
      size: 100MB

    prod-configs:
      cloud: gce
      type: ssd
      size: 100MB

    prod-db:
      cloud: gce
      type: ssd
      size: 100GB

  instance:

    gce-host-0:
      cloud: gce
      ip: 127:0:1:1
      labels: prod, prod-mongodb
      mounts:
        config: prod-configs => /config
        db: prod-db => /data

    gce-host-1:
      cloud: gce
      machine-type: n1-standard-1
      zone: us-west
      ip: 127:0:0:1
      labels: prod, lb
      mounts:
        config: prod-configs => /config

    gce-host-2:
      cloud: gce
      ip: 127:0:1:1
      labels: dev, stage, db
      mounts:
        config: dev-config => /config
        db: dev-mongodb => /data
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
