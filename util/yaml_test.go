package util

import (
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v1"
	"testing"
)

const test_yml = `
deploy:
  - passport-prod
  - shorty-prod

import:
  - resources.yml
  - instances.yml

var:
  CIRCLECI_API_TOKEN: b71701145614b93a382a8e3b5d633ee71c360315c
  DOCKER_ACCOUNT: lab616
  DOCKER_EMAIL: davidc616@gmail.com
  DOCKER_AUTH: bGFiNjE2OmxhYjYxNgc==
  BUILD_NUMBER: 292
  PASSPORT_IMAGE_TAG: 292
  SHORTY_IMAGE_TAG: 292

service:

  passport:
      - mongodb: db
      - passport: dev

  passport-prod:
      - mongodb-prod: prod-db
      - passport: prod
      - nginx: lb

artifact:
  passport:
    project: qorio/omni
    server: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: passport
    platform: linux_amd64

  shorty:
    project: qorio/omni
    server: circleci
    build_number: {{.BUILD_NUMBER}}
    platform: linux_amd64
    artifact: shorty

  geoip:
    project: qorio/omni
    server: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: GeoLiteCity.dat

docker:
  passport:
     dockerfile: docker/passport/Dockerfile
     image: qorio/passport:{{.PASSPORT_IMAGE_TAG}}
     artifacts:
       - passport

  shorty:
    dockerfile: docker/shorty/Dockerfile
    image: qorio/shorty:{{.SHORTY_IMAGE_TAG}}
    artifacts:
      - geoip
      - shorty

# docker objects reference the build of the same name.
# also, the host object is mapped to the scope of the docker object so host mounts are referenceable
container:
  passport:
      ssh:
         - docker run -d -p 5050:5050 -v {{.host.volumes.config}}/omni:/static/conf --name passport {{.image}}

  shorty:
      ssh:
         - docker run -d -p 5050:5050 -v {{.host.volumes.config}}/omni:/static/conf --name shorty {{.image}}

  mongodb:
      image: mongo:2.7.5
      ssh:
        - docker run -d -p 27017:27017 -v {{.host.volumes.db}}/mongo:/data/db --name mongodb {{.image}}

  redis:
      image: redis:2.8.13
      ssh:
        - docker run -d -p 6379:6379 -v {{.host.volumes.db}}:/data
          -v {{.host.volumes.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  redis-prod:
      image: redis:2.8.13
      ssh:
        - >
          docker run -d -p 6379:6379 -v {{.resource.disk.redisdb-prod.mount}}:/data
           -v {{.host.volumes.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  nginx:
      image: nginx:1.7.1
      ssh:
        - docker run -d -v /some/nginx.conf:/etc/nginx.conf:ro --name nginx {{.image}}


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
      project: qoriolabsdev
      internal-ip: 127:0:1:1
      labels: prod, prod-mongodb
      volumes:
        config: prod-configs => /config
        db: prod-db => /data

    gce-host-1:
      cloud: gce
      project: qoriolabsdev
      machine-type: n1-standard-1
      zone: us-west
      internal-ip: 127:0:0:1
      labels: prod, lb
      volumes:
        config: prod-configs => /config

    gce-host-2:
      cloud: gce
      project: qoriolabsdev
      internal-ip: 127:0:1:1
      labels: dev, stage, db
      volumes:
        config: dev-config => /config
        db: dev-mongodb => /data
`

func TestYaml(t *testing.T) { TestingT(t) }

type YamlTests struct{}

var _ = Suite(&YamlTests{})

func (suite *YamlTests) TestLoadArtifactSection(c *C) {

	doc := struct {
		Import   []string
		Deploy   []string
		Var      map[string]string
		Service  map[string][]map[string]string
		Artifact map[string]struct {
			Project     string
			Server      string
			BuildNumber int
			Artifact    string
			Platform    string
		}
		Docker map[string]struct {
			Dockerfile string
			Image      string
			Artifacts  []string
		}
		Container map[string]struct {
			Image string
			Ssh   []string
		}
		Resource struct {
			Disk map[string]struct {
				Cloud string
				Type  string
				Size  string
			}
			Instance map[string]struct {
				Cloud      string
				Project    string
				InternalIp string `yaml:"internal-ip"`
				ExternalIp string `yaml:"external-ip"`
				Labels     string
				Volumes    map[string]string
			}
		}
	}{}

	err := yaml.Unmarshal([]byte(test_yml), &doc)
	c.Assert(err, Equals, nil)

	c.Assert(len(doc.Deploy), Equals, 2)

	c.Assert(len(doc.Import), Equals, 2)

	c.Assert(len(doc.Var), Equals, 7)
	c.Assert(doc.Var["DOCKER_AUTH"], Equals, "bGFiNjE2OmxhYjYxNgc==")

	artifacts := doc.Artifact
	c.Assert(len(artifacts), Equals, 3)
	c.Assert(artifacts["shorty"].Artifact, Equals, "shorty")

	dockers := doc.Docker
	c.Assert(len(dockers), Equals, 2)
	c.Assert(dockers["shorty"].Artifacts[1], Equals, "shorty")

	containers := doc.Container
	c.Assert(len(containers), Equals, 6)
	c.Assert(len(containers["redis"].Ssh), Equals, 1)
	c.Assert(containers["nginx"].Image, Equals, "nginx:1.7.1")

	service := doc.Service
	c.Assert(service["passport"][0]["mongodb"], Equals, "db")
	c.Assert(service["passport"][1]["passport"], Equals, "dev")

	c.Assert(service["passport-prod"][0]["mongodb-prod"], Equals, "prod-db")
	c.Assert(service["passport-prod"][1]["passport"], Equals, "prod")
	c.Assert(service["passport-prod"][2]["nginx"], Equals, "lb")

	disks := doc.Resource.Disk
	c.Assert(len(disks), Equals, 4)
	c.Assert(disks["dev-configs"].Cloud, Equals, "gce")

	instances := doc.Resource.Instance
	c.Assert(len(instances), Equals, 3)
	c.Assert(instances["gce-host-2"].Volumes["config"], Equals, "dev-config => /config")
}
