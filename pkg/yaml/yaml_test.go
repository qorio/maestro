package yaml

import (
	. "gopkg.in/check.v1"
	"os"
	"path/filepath"
	"testing"
)

const yml = `
import:
#  - $HOME/resources.yml
#  - $HOME/instances.yml

var:
  LIVE_MODE: 0
  DOCKER_ACCOUNT: qoriolabs
  DOCKER_EMAIL: docker@qoriolabs.com
  DOCKER_AUTH: cW9yaW9sYWJzOlFvcmlvMWxhYnMh
  BUILD_NUMBER: 12
  DOCKER_DIR: $HOME/go/src/github.com/qorio/maestro/docker
  KEY_DIR: $HOME/go/src/github.com/qorio/maestro/environments/dev/.ssh

deploy:
  - passport

artifact:
  auth_key:
    project: qorio/passport
    source: circleci
    source-api-token: ea735505e0fae755201ea1511c4fa9c55f825846
    build: "{{.BUILD_NUMBER}}"
    artifact: testAuthKey.pub

  passport:
    project: qorio/passport
    source: circleci
    source-api-token: ea735505e0fae755201ea1511c4fa9c55f825846
    build: "{{.BUILD_NUMBER}}"
    artifact: passport

image:
  passport:
     dockerfile: "{{.DOCKER_DIR}}/passport/Dockerfile"
     image: "{{.DOCKER_ACCOUNT}}/passport:{{.BUILD_NUMBER}}"
     artifacts:
       - passport

container:
  mongodb:
      image: mongo:2.7.5
      ssh:
        # Name the container as mongo
        - docker run -d -p 27017:27017 -v {{.instance.volumes.db.mount}}/mongo:/data/db --name mongodb {{.image.id}}
  passport:
      image: passport
      ssh:
         - echo "Host {{.instance.name}} running {{.image.id}} build {{.BUILD_NUMBER}}"
         # Use container linking to reference the mongo container (see above), which is mapped in /etc/host as 'mongodb'
         - docker run -d -p 5050:5050 -v {{.instance.volumes.config.mount}}:/static/conf:ro --link mongo:mongodb --name passport_{{.BUILD_NUMBER}} {{.image.id}}
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

job:
  mongodb:
      container: mongodb
      instances: db
  passport:
      container: passport
      instances: dev

service:
  passport:
      - mongodb: db
      - passport: dev

resource:
  disk:
    dev_config:
      cloud: gce
      type: disk
      size: 100MB
    dev_db:
      cloud: gce
      type: disk
      size: 100MB

  instance:
    gce-host-0:
      keypair: "{{.KEY_DIR}}/gce-qoriolabsdev"
      cloud: gce
      project: qoriolabsdev
      internal-ip: 192.30.252.154
      external-ip: 164.77.100.101
      labels:
        - dev
        - db
      volumes:
        config:
           dev_config: /config
        db:
           dev_db: /data
    gce-host-1:
      keypair: "{{.KEY_DIR}}/gce-qoriolabsdev"
      cloud: gce
      project: qoriolabsdev
      internal-ip: 192.30.252.155
      external-ip: 164.77.100.102
      labels:
        - dev
      volumes:
        config:
           dev_config: /config
`

func TestYaml(t *testing.T) { TestingT(t) }

type YamlTests struct{}

var _ = Suite(&YamlTests{})

func (suite *YamlTests) TestTestModeCheck(c *C) {
	m := make(Context)
	m[LIVE_MODE] = "true"
	c.Assert(m.test_mode(), Equals, false)

	m[LIVE_MODE] = "1"
	c.Assert(m.test_mode(), Equals, false)

	m[LIVE_MODE] = "TRUE"
	c.Assert(m.test_mode(), Equals, false)

	m[LIVE_MODE] = "junk"
	c.Assert(m.test_mode(), Equals, true)

	m[LIVE_MODE] = 1
	c.Assert(m.test_mode(), Equals, false)

	m[LIVE_MODE] = 0
	c.Assert(m.test_mode(), Equals, true)

	m[LIVE_MODE] = "0"
	c.Assert(m.test_mode(), Equals, true)

	m[LIVE_MODE] = "false"
	c.Assert(m.test_mode(), Equals, true)

	m[LIVE_MODE] = "FALSE"
	c.Assert(m.test_mode(), Equals, true)

	delete(m, LIVE_MODE)
	c.Assert(m.test_mode(), Equals, true)
}

func (suite *YamlTests) TestSchema(c *C) {

	doc := MaestroDoc{}
	err := doc.LoadFromBytes([]byte(yml))
	c.Assert(err, Equals, nil)

	c.Assert(len(doc.Deploys), Equals, 1)
	c.Assert(len(doc.Imports), Equals, 0) // commented out

	c.Assert(len(doc.Vars), Equals, 7)
	c.Assert(doc.Vars["DOCKER_AUTH"], Equals, "cW9yaW9sYWJzOlFvcmlvMWxhYnMh")

	artifacts := doc.Artifacts
	c.Assert(len(artifacts), Equals, 2)

	images := doc.Images
	c.Assert(len(images), Equals, 1)
	c.Assert(images["passport"].ArtifactKeys[0], Equals, ArtifactKey("passport"))

	containers := doc.Containers
	c.Assert(len(containers), Equals, 5)
	c.Assert(len(containers["redis"].Ssh), Equals, 1)
	c.Assert(containers["nginx"].ImageRef, Equals, "nginx:1.7.1")

	service := doc.ServiceSection
	c.Assert(service["passport"][0]["mongodb"], Equals, InstanceLabel("db"))
	c.Assert(service["passport"][1]["passport"], Equals, InstanceLabel("dev"))

	instances := doc.Resources.Instances
	c.Assert(len(instances), Equals, 2)
	c.Assert(instances["gce-host-1"].InternalIp, Equals, Ip("192.30.252.155"))

	disks := doc.Resources.Disks
	c.Assert(len(disks), Equals, 2)
	c.Assert(disks["dev_config"].Cloud, Equals, "gce")
}

func (suite *YamlTests) TestProcessImages(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(`
artifact:
  passport:
    project: qorio/omni
    source: circleci
    build: "{{.BUILD_NUMBER}}"
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
	c.Assert(config.Images["passport"].artifacts[0].BuildNumber, Equals, "{{.BUILD_NUMBER}}")
	c.Assert(string(config.Images["passport"].artifacts[0].Name), Equals, "passport")
	c.Assert(string(config.Images["passport"].RepoId), Equals, "qorio/passport:{{.PASSPORT_IMAGE_TAG}}")

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
    build: {{.BUILD_NUMBER}}
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
	c.Assert(config.Containers["passport"].TargetImage, Not(Equals), nil)
	c.Assert(config.Containers["passport"].TargetImage, Equals, config.Images["passport"])
	c.Assert(config.Containers["passport"].TargetImage.Dockerfile, Equals, "docker/passport/Dockerfile")
	c.Assert(len(config.Containers["passport"].Ssh), Equals, 2)

	c.Assert(config.Containers["mongodb"].TargetImage, Equals, (*Image)(nil))
	c.Assert(len(config.Containers["mongodb"].Ssh), Equals, 1)
}

func (suite *YamlTests) TestVariableSubstitution(c *C) {
	x := make(Context)
	x["foo"] = "bar"

	a := &Artifact{
		Project: "project-{{.foo}}",
	}

	old := x.eval(&a.Project)
	c.Assert(old, Equals, "project-{{.foo}}")
	c.Assert(a.Project, Equals, "project-bar")
}

func (suite *YamlTests) TestValidate(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(yml))
	c.Assert(err, Equals, nil)

	err = config.process_config()
	c.Assert(err, Equals, nil)

	c.Assert(len(config.Services), Equals, 1)
	c.Assert(config.Services["passport"].Name, Equals, ServiceKey("passport"))
	c.Assert(len(config.Services["passport"].Targets), Equals, 2)
	c.Assert(len(config.Services["passport"].Targets[0]), Equals, 1) // 1 vm
	c.Assert(len(config.Services["passport"].Targets[1]), Equals, 2) // 2 vms

	db := config.Services["passport"].Targets[0][0]
	c.Assert(string(db.Name), Equals, "mongodb")
	c.Assert(string(db.ImageRef), Equals, "mongo:2.7.5")
	c.Assert(db.TargetInstance, Equals, config.Resources.Instances["gce-host-0"])
	c.Assert(db.TargetImage, Equals, config.Images["mongodb"])

	fe := config.Services["passport"].Targets[1]
	c.Assert(len(fe), Equals, 2)
	c.Assert(fe[0].TargetImage, Equals, config.Images["passport"])
	c.Assert(fe[1].TargetImage, Equals, config.Images["passport"])
	c.Assert(fe[0].TargetInstance, Not(Equals), (*Instance)(nil))
	c.Assert(fe[1].TargetInstance, Not(Equals), (*Instance)(nil))
	c.Assert(fe[0].TargetInstance, Not(Equals), fe[1].TargetInstance)
	c.Assert(fe[0].TargetInstance, Equals, config.Resources.Instances["gce-host-0"])
	c.Assert(fe[1].TargetInstance, Equals, config.Resources.Instances["gce-host-1"])

	// now validate
	context := config.new_context()
	err = config.Validate(context)
	c.Assert(err, Equals, nil)

	// After Validate, all variable substitutions should be completed.

	// If no errors, check the state of the service
	passport_service := config.Services[ServiceKey("passport")]
	c.Assert(passport_service.Name, Equals, ServiceKey("passport"))

	mongo_db_containers := passport_service.Targets[0]
	c.Assert(len(mongo_db_containers), Equals, 1)
	db1 := mongo_db_containers[0]
	c.Assert(db1.TargetImage, Equals, (*Image)(nil))
	c.Assert(db1.ImageRef, Equals, "mongo:2.7.5")
	c.Assert(*db1.Ssh[0], Equals, "docker run -d -p 27017:27017 -v /data/mongo:/data/db --name mongodb mongo:2.7.5")

	passport_containers := passport_service.Targets[1]
	c.Assert(len(passport_containers), Equals, 2)

	s1 := passport_containers[0]
	c.Log(s1.TargetImage.Dockerfile)
	c.Assert(s1.TargetInstance.ExternalIp, Equals, Ip("164.77.100.101"))
	c.Assert(*s1.Ssh[0], Equals, "echo \"Host gce-host-0 running qoriolabs/passport:12 build 12\"")
	c.Assert(*s1.Ssh[1], Equals, "docker run -d -p 5050:5050 -v /config:/static/conf:ro --link mongo:mongodb --name passport_12 qoriolabs/passport:12")

	s2 := passport_containers[1]
	c.Assert(s2.TargetInstance.ExternalIp, Equals, Ip("164.77.100.102"))
	c.Assert(*s2.Ssh[1], Equals, "docker run -d -p 5050:5050 -v /config:/static/conf:ro --link mongo:mongodb --name passport_12 qoriolabs/passport:12")

}

func (suite *YamlTests) TestPrepareImages(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(yml))
	c.Assert(err, Equals, nil)

	err = config.process_config()
	c.Assert(err, Equals, nil)

	context := config.new_context()
	err = config.Validate(context)
	c.Assert(err, Equals, nil)

	err = config.runnableImages().Prepare(context)
	c.Assert(err, Equals, nil)

	// Check downloaded binary exists
	_, err = os.Stat(filepath.Dir(config.Images["passport"].Dockerfile) + "/passport")
	c.Assert(err, Equals, nil)
}

func (suite *YamlTests) TestExecuteImages(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(yml))
	c.Assert(err, Equals, nil)

	err = config.process_config()
	c.Assert(err, Equals, nil)

	context := config.new_context()
	err = config.Validate(context)
	c.Assert(err, Equals, nil)

	err = config.runnableImages().Prepare(context)
	c.Assert(err, Equals, nil)

	err = config.runnableImages().Execute(context)
	c.Assert(err, Equals, nil)
}
