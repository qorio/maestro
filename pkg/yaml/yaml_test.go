package yaml

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
    source: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: passport
    platform: linux_amd64

  shorty:
    project: qorio/omni
    source: circleci
    build_number: {{.BUILD_NUMBER}}
    platform: linux_amd64
    artifact: shorty

  geoip:
    project: qorio/omni
    source: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: GeoLiteCity.dat

  dev-keys:
    source: local
    path: dir/to/key

image:
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
      image: passport
      ssh:
         - docker run -d -p 5050:5050 -v {{.instance.volumes.config}}/omni:/static/conf --name passport {{.image}}

  shorty:
      image: shorty
      ssh:
         - docker run -d -p 5050:5050 -v {{.instance.volumes.config}}/omni:/static/conf --name shorty {{.image}}

  mongodb:
      image: mongo:2.7.5
      ssh:
        - docker run -d -p 27017:27017 -v {{.instance.volumes.db}}/mongo:/data/db --name mongodb {{.image}}

  redis:
      image: redis:2.8.13
      ssh:
        - docker run -d -p 6379:6379 -v {{.instance.volumes.db}}:/data
          -v {{.instance.volumes.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

  redis-prod:
      image: redis:2.8.13
      ssh:
        - >
          docker run -d -p 6379:6379 -v {{.resource.disk.redisdb-prod.mount}}:/data
           -v {{.instance.volumes.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server

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
      internal-ip: 127.0.0.1
      labels:
        - prod
        - prod-mongodb
      volumes:
        config:
           /config: prod-configs
        db:
           /data: prod-db

    gce-host-1:
      cloud: gce
      project: qoriolabsdev
      machine-type: n1-standard-1
      zone: us-west
      internal-ip: 127.0.0.1
      labels:
        - prod
        - lb
      volumes:
        config:
          /config: prod-configs

    gce-host-2:
      cloud: gce
      project: qoriolabsdev
      internal-ip: 127.0.0.1
      labels:
        - dev
        - stage
        - db
      volumes:
        config:
          /config: dev-config
        data:
          /data: dev-mongodb
`

func TestYaml(t *testing.T) { TestingT(t) }

type YamlTests struct{}

var _ = Suite(&YamlTests{})

func (suite *YamlTests) TestSchema(c *C) {

	doc := MaestroDoc{}

	err := yaml.Unmarshal([]byte(test_yml), &doc)
	c.Assert(err, Equals, nil)

	c.Assert(len(doc.Deploys), Equals, 2)

	c.Assert(len(doc.Imports), Equals, 2)

	c.Assert(len(doc.Vars), Equals, 7)
	c.Assert(doc.Vars["DOCKER_AUTH"], Equals, "bGFiNjE2OmxhYjYxNgc==")

	artifacts := doc.Artifacts
	c.Assert(len(artifacts), Equals, 4)
	c.Assert(artifacts["shorty"].Artifact, Equals, "shorty")

	images := doc.Images
	c.Assert(len(images), Equals, 2)
	c.Assert(images["shorty"].ArtifactKeys[1], Equals, ArtifactKey("shorty"))

	containers := doc.Containers
	c.Assert(len(containers), Equals, 6)
	c.Assert(len(containers["redis"].Ssh), Equals, 1)
	c.Assert(containers["nginx"].ImageRef, Equals, "nginx:1.7.1")

	service := doc.ServiceSection
	c.Assert(service["passport"][0]["mongodb"], Equals, InstanceLabel("db"))
	c.Assert(service["passport"][1]["passport"], Equals, InstanceLabel("dev"))

	c.Assert(service["passport-prod"][0]["mongodb-prod"], Equals, InstanceLabel("prod-db"))
	c.Assert(service["passport-prod"][1]["passport"], Equals, InstanceLabel("prod"))
	c.Assert(service["passport-prod"][2]["nginx"], Equals, InstanceLabel("lb"))

	instances := doc.Resources.Instances
	c.Assert(len(instances), Equals, 3)
	c.Assert(instances["gce-host-2"].InternalIp, Equals, Ip("127.0.0.1"))
	c.Assert(instances["gce-host-2"].Volumes["config"]["/config"], Equals, DiskKey("dev-config"))

	disks := doc.Resources.Disks
	c.Assert(len(disks), Equals, 4)
	c.Assert(disks["dev-configs"].Cloud, Equals, "gce")
}

func (suite *YamlTests) TestProcessImages(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(`
artifact:
  passport:
    project: qorio/omni
    source: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: passport
    platform: linux_amd64

image:
  passport:
     dockerfile: docker/passport/Dockerfile
     image: qorio/passport:{{.PASSPORT_IMAGE_TAG}}
     artifacts:
       - passport
`))
	c.Assert(err, Equals, nil)

	err = config.process_images()
	c.Assert(err, Equals, nil)

	c.Assert(len(config.Images), Equals, 1)
	c.Assert(len(config.Images["passport"].ArtifactKeys), Equals, 1)
	c.Assert(config.Images["passport"].artifacts, Not(Equals), nil)
	c.Assert(config.Images["passport"].artifacts[0].Platform, Equals, "linux_amd64")
	c.Assert(config.Images["passport"].artifacts[0].Source, Equals, "circleci")
	c.Assert(config.Images["passport"].artifacts[0].Artifact, Equals, "passport")
	c.Assert(string(config.Images["passport"].artifacts[0].Name), Equals, "passport")

}

func (suite *YamlTests) TestProcessContainers(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(`
var:
  PASSPORT_IMAGE_TAG: 292

artifact:
  passport:
    project: qorio/omni
    source: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: passport
    platform: linux_amd64

image:
  passport:
     dockerfile: docker/passport/Dockerfile
     image: qorio/passport:{{.PASSPORT_IMAGE_TAG}}
     artifacts:
       - passport

container:
  passport:
      image: passport
      ssh:
         - docker stop --name passport
         - docker run -d -p 5050:5050 -v {{.instance.volumes.config}}/omni:/static/conf --name passport {{.image}}
  mongodb:
      image: mongo:2.7.5
      ssh:
        - docker run -d -p 27017:27017 -v {{.instance.volumes.db}}/mongo:/data/db --name mongodb {{.image}}
`))
	c.Assert(err, Equals, nil)

	err = config.process_images()
	c.Assert(err, Equals, nil)

	err = config.process_containers()
	c.Assert(err, Equals, nil)

	c.Assert(len(config.Containers), Equals, 2)
	c.Assert(config.Containers["passport"].image, Not(Equals), nil)
	c.Assert(config.Containers["passport"].image, Equals, config.Images["passport"])
	c.Assert(config.Containers["passport"].image.Dockerfile, Equals, "docker/passport/Dockerfile")
	c.Assert(len(config.Containers["passport"].Ssh), Equals, 2)

	c.Assert(config.Containers["mongodb"].image, Equals, (*Image)(nil))
	c.Assert(len(config.Containers["mongodb"].Ssh), Equals, 1)
}

func (suite *YamlTests) TestBuildServices(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(`
var:
  PASSPORT_IMAGE_TAG: 292

artifact:
  passport:
    project: qorio/omni
    source: circleci
    build_number: {{.BUILD_NUMBER}}
    artifact: passport
    platform: linux_amd64

image:
  passport:
     dockerfile: docker/passport/Dockerfile
     image: qorio/passport:{{.PASSPORT_IMAGE_TAG}}
     artifacts:
       - passport

container:
  passport:
      image: passport
      ssh:
         - docker stop --name passport
         - docker run -d -p 5050:5050 -v {{.instance.volumes.config}}/omni:/static/conf --name passport {{.image}}
  mongodb:
      image: mongo:2.7.5
      ssh:
        - docker run -d -p 27017:27017 -v {{.instance.volumes.db}}/mongo:/data/db --name mongodb {{.image}}

service:
  passport:
      - mongodb: db
      - passport: dev

resource:
  disk:
    dev-config:
      cloud: gce
      type: disk
      size: 100MB
    dev-db:
      cloud: gce
      type: disk
      size: 100MB

  instance:
    gce-host-0:
      cloud: gce
      project: qoriolabsdev
      internal-ip: 192.30.252.154
      external-ip: 164.77.100.101
      labels:
        - dev
        - db
      volumes:
        config:
           /config: dev-config
        db:
           /data: dev-db
    gce-host-1:
      cloud: gce
      project: qoriolabsdev
      internal-ip: 192.30.252.155
      external-ip: 164.77.100.102
      labels:
        - dev
      volumes:
        config:
           /config: dev-config
`))
	c.Assert(err, Equals, nil)

	err = config.process_images()
	c.Assert(err, Equals, nil)

	err = config.process_containers()
	c.Assert(err, Equals, nil)

	err = config.process_resources()
	c.Assert(err, Equals, nil)

	err = config.build_services()
	c.Assert(err, Equals, nil)

	c.Assert(len(config.Services), Equals, 1)
	c.Assert(config.Services["passport"].Name, Equals, ServiceKey("passport"))
	c.Assert(len(config.Services["passport"].Targets), Equals, 2)
	c.Assert(len(config.Services["passport"].Targets[0]), Equals, 1) // 1 vm
	c.Assert(len(config.Services["passport"].Targets[1]), Equals, 2) // 2 vms

	db := config.Services["passport"].Targets[0][0]
	c.Assert(string(db.Name), Equals, "mongodb")
	c.Assert(string(db.ImageRef), Equals, "mongo:2.7.5")
	c.Assert(db.instance, Equals, config.Resources.Instances["gce-host-0"])
	c.Assert(db.image, Equals, config.Images["mongodb"])

	fe := config.Services["passport"].Targets[1]
	c.Assert(len(fe), Equals, 2)
	c.Assert(fe[0].image, Equals, config.Images["passport"])
	c.Assert(fe[1].image, Equals, config.Images["passport"])
	c.Assert(fe[0].instance, Not(Equals), (*Instance)(nil))
	c.Assert(fe[1].instance, Not(Equals), (*Instance)(nil))
	c.Assert(fe[0].instance, Not(Equals), fe[1].instance)
	c.Assert(fe[0].instance, Equals, config.Resources.Instances["gce-host-0"])
	c.Assert(fe[1].instance, Equals, config.Resources.Instances["gce-host-1"])

}
