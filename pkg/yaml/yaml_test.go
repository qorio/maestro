package yaml

import (
	"fmt"
	. "gopkg.in/check.v1"
	"os"
	"path/filepath"
	"testing"
)

const disks = `
disk:
    dev_config:
      cloud: gce
      type: disk
      size: 100M
    dev_db:
      cloud: gce
      type: disk
      size: 100G
`
const instances = `
instance:
    gce-host-0:
      available:
           cpu: 2
           memory: 4G
           disk: 10G  # ephemeral disk
      keypair: "{{.KEY_DIR}}/gce-qoriolabsdev"
      cloud: gce
      project: qoriolabsdev
      internal-ip: 192.30.252.154
      external-ip: 164.77.100.101
      labels: dev, db
      volumes:
        config:
           dev_config: /config
        db:
           dev_db: /data
    gce-host-1:
      available:
           cpu: 2
           memory: 4G
           disk: 10G  # ephemeral disk
      keypair: "{{.KEY_DIR}}/gce-qoriolabsdev"
      cloud: gce
      project: qoriolabsdev
      internal-ip: 192.30.252.155
      external-ip: 164.77.100.102
      labels: dev
      volumes:
        config:
           dev_config: /config
`

const images_yml = `
var:
  CIRCLECI_TOKEN: ea735505e0fae755201ea1511c4fa9c55f825846
  DOCKER_ACCOUNT: qoriolabs
  DOCKER_EMAIL: docker@qoriolabs.com
  DOCKER_AUTH: cW9yaW9sYWJzOlFvcmlvMWxhYnMh
  BUILD_NUMBER: 12
  DOCKER_DIR: $HOME/go/src/github.com/qorio/maestro/docker

artifact:
  passport:
    project: qorio/passport
    source: circleci
    source-api-token: '{{.CIRCLECI_TOKEN}}'
    build: "{{.BUILD_NUMBER}}"
    file: passport

  auth_key:
    project: qorio/passport
    source: circleci
    source-api-token: '{{.CIRCLECI_TOKEN}}'
    build: "{{.BUILD_NUMBER}}"
    file: authKey.pub

image:
  passport:
     dockerfile: "{{.DOCKER_DIR}}/passport/Dockerfile"
     image: "{{.DOCKER_ACCOUNT}}/passport:{{.BUILD_NUMBER}}"
     artifacts:
       - passport
       - auth_key
`

const yml = `
# Imports here
# import:
#   - $HOME/file1.yml
#   - $HOME/file2.yml

var:
  LIVE_MODE: 0
  KEY_DIR: $HOME/go/src/github.com/qorio/maestro/environments/dev/.ssh

deploy:
  - passport

container:
  mongodb:
      image: mongo:2.7.5
      requires:
           cpu: 1
           memory: 1G
           disk: 200G
      ssh:
        # Name the container as mongo
        - docker run -d -p 27017:27017 -v {{.instance.volumes.db.mount}}/mongo:/data/db --name mongodb {{.image.id}}
  passport:
      image: passport
      requires:
           cpu: 1
           memory: 1G
           disk: 10G
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
          docker run -d -p 6379:6379 -v {{.resource.disk.dev_db.mount}}:/data
           -v {{.instance.volumes.config}}/redis/redis.conf:/etc/redis/redis.conf:ro --name redis {{.image}} redis-server
  nginx:
      image: nginx:1.7.1
      ssh:
        - docker run -d -v /some/nginx.conf:/etc/nginx.conf:ro --name nginx {{.image}}

job:
  mongodb:
      container: mongodb
      instance-labels: db
      requires:
           cpu: 16
           memory: 1G
           disk: 200G

  passport:
      container: passport
      instance-labels: dev, db

service:
  passport:
     # These will be launched in order
     - mongodb: 27017
     - passport: 80, 7070
`

func TestYaml(t *testing.T) { TestingT(t) }

type YamlTests struct {
	dir            string
	config_file    string
	disks_file     string
	instances_file string
	images_file    string
}

var _ = Suite(&YamlTests{})

func (suite *YamlTests) SetUpSuite(c *C) {
	// Write the resources file out to disk
	suite.dir = c.MkDir()
	disks_file, err := os.Create(filepath.Join(suite.dir, "disks.yml"))
	if err != nil {
		panic(err)
	}
	defer disks_file.Close()
	suite.disks_file = disks_file.Name()

	len, err := fmt.Fprintln(disks_file, disks)
	if err != nil {
		panic(err)
	}
	c.Log("Generated disks file: ", len)

	instances_file, err := os.Create(filepath.Join(suite.dir, "instances.yml"))
	if err != nil {
		panic(err)
	}
	defer instances_file.Close()
	suite.instances_file = instances_file.Name()

	len, err = fmt.Fprintln(instances_file, instances)
	if err != nil {
		panic(err)
	}
	c.Log("Generated instances file: ", len)

	images_file, err := os.Create(filepath.Join(suite.dir, "images.yml"))
	if err != nil {
		panic(err)
	}
	defer images_file.Close()
	suite.images_file = images_file.Name()

	len, err = fmt.Fprintln(images_file, images_yml)
	if err != nil {
		panic(err)
	}
	c.Log("Generated images file: ", len)

	// Now write the yml file and
	config_file, err := os.Create(filepath.Join(suite.dir, "config.yml"))
	if err != nil {
		panic(err)
	}
	defer config_file.Close()
	suite.config_file = config_file.Name()

	fmt.Fprintln(config_file, `
import:
   - `+suite.disks_file)
	fmt.Fprintln(config_file, `
   - `+suite.instances_file)
	fmt.Fprintln(config_file, `
   - `+suite.images_file)

	fmt.Fprintln(config_file, yml)
	c.Log("Test Setup:", suite)
}

func (suite *YamlTests) TearDownSuite(c *C) {

}

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
	c.Log("Reading from", suite.config_file)
	err := doc.LoadFromFile(suite.config_file)
	c.Assert(err, Equals, nil)

	c.Log("FINAL", doc.String())

	c.Assert(len(doc.Deploys), Equals, 1)
	c.Assert(len(doc.Imports), Equals, 3)

	c.Assert(len(doc.Vars), Equals, 8)
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
	c.Assert(service["passport"][0]["mongodb"], Equals, JobPortList("27017"))
	c.Assert(service["passport"][1]["passport"], Equals, JobPortList("80, 7070"))

	instances := doc.Instances
	c.Assert(len(instances), Equals, 2)
	c.Assert(instances["gce-host-1"].InternalIp, Equals, Ip("192.30.252.155"))

	disks := doc.Disks
	c.Assert(len(disks), Equals, 2)
	c.Assert(disks["dev_config"].Cloud, Equals, "gce")

	jobs := doc.Jobs
	c.Assert(len(jobs), Equals, 2)
	c.Assert(jobs["mongodb"].ContainerKey, Equals, ContainerKey("mongodb"))
	c.Assert(jobs["passport"].ContainerKey, Equals, ContainerKey("passport"))
	c.Assert(jobs["mongodb"].InstanceLabels, Equals, InstanceLabelList("db"))
	c.Assert(jobs["passport"].InstanceLabels, Equals, InstanceLabelList("dev, db"))

}

func (suite *YamlTests) TestProcessImages(c *C) {

	config := &MaestroDoc{}
	err := config.LoadFromBytes([]byte(`
artifact:
  passport:
    project: qorio/omni
    source: circleci
    build: "{{.BUILD_NUMBER}}"
    file: passport
    platform: linux_amd64

image:
  passport:
     dockerfile: docker/passport/Dockerfile
     image: qorio/passport:{{.PASSPORT_IMAGE_TAG}}
     artifacts:
       - passport
`))
	c.Assert(err, Equals, nil)

	err = config.process_artifacts()
	c.Assert(err, Equals, nil)

	err = config.process_images()
	c.Assert(err, Equals, nil)

	c.Assert(len(config.Images), Equals, 1)
	c.Assert(len(config.Images["passport"].ArtifactKeys), Equals, 1)
	c.Assert(config.Images["passport"].artifacts, Not(Equals), nil)
	c.Assert(config.Images["passport"].artifacts[0].Platform, Equals, "linux_amd64")
	c.Assert(config.Images["passport"].artifacts[0].Source, Equals, "circleci")
	c.Assert(config.Images["passport"].artifacts[0].File, Equals, "passport")
	c.Assert(config.Images["passport"].artifacts[0].BuildNumber, Equals, "{{.BUILD_NUMBER}}")
	c.Assert(string(config.Images["passport"].artifacts[0].name), Equals, "passport")
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
    file: passport
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
	c.Assert(config.Containers["passport"].targetImage, Not(Equals), nil)
	c.Assert(config.Containers["passport"].targetImage, Equals, config.Images["passport"])
	c.Assert(config.Containers["passport"].targetImage.Dockerfile, Equals, "docker/passport/Dockerfile")
	c.Assert(len(config.Containers["passport"].Ssh), Equals, 2)

	c.Assert(config.Containers["mongodb"].targetImage, Equals, (*Image)(nil))
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

	config := MaestroDoc{}
	c.Log("Reading from", suite.config_file)
	err := config.LoadFromFile(suite.config_file)
	c.Assert(err, Equals, nil)

	err = config.process_config()
	c.Assert(err, Equals, nil)

	c.Assert(len(config.services), Equals, 1)
	c.Assert(config.services["passport"].Name(), Equals, ServiceKey("passport"))
	c.Assert(len(config.services["passport"].Targets()), Equals, 2)
	c.Assert(len(config.services["passport"].Targets()[0]), Equals, 1) // 1 vm
	c.Assert(len(config.services["passport"].Targets()[1]), Equals, 2) // 2 vms

	db := config.services["passport"].Targets()[0][0]
	c.Assert(string(db.name), Equals, "mongodb")
	c.Assert(string(db.ImageRef), Equals, "mongo:2.7.5")
	c.Assert(db.targetInstance, Equals, config.Instances["gce-host-0"])
	c.Assert(db.targetImage, Equals, config.Images["mongodb"])

	fe := config.services["passport"].Targets()[1]
	c.Assert(len(fe), Equals, 2)
	c.Assert(fe[0].targetImage, Equals, config.Images["passport"])
	c.Assert(fe[1].targetImage, Equals, config.Images["passport"])
	c.Assert(fe[0].targetInstance, Not(Equals), (*Instance)(nil))
	c.Assert(fe[1].targetInstance, Not(Equals), (*Instance)(nil))
	c.Assert(fe[0].targetInstance, Not(Equals), fe[1].targetInstance)
	c.Assert(fe[0].targetInstance, Equals, config.Instances["gce-host-0"])
	c.Assert(fe[1].targetInstance, Equals, config.Instances["gce-host-1"])

	// now validate
	context := config.new_context()
	err = config.Validate(context)
	c.Assert(err, Equals, nil)

	// After Validate, all variable substitutions should be completed.

	// If no errors, check the state of the service
	passport_service := config.services[ServiceKey("passport")]
	c.Assert(passport_service.Name(), Equals, ServiceKey("passport"))

	mongo_db_containers := passport_service.Targets()[0]
	c.Assert(len(mongo_db_containers), Equals, 1)
	db1 := mongo_db_containers[0]
	c.Assert(db1.targetImage, Equals, (*Image)(nil))
	c.Assert(db1.ImageRef, Equals, "mongo:2.7.5")
	c.Assert(*db1.Ssh[0], Equals, "docker run -d -p 27017:27017 -v /data/mongo:/data/db --name mongodb mongo:2.7.5")

	passport_containers := passport_service.Targets()[1]
	c.Assert(len(passport_containers), Equals, 2)

	s1 := passport_containers[0]
	c.Log(s1.targetImage.Dockerfile)
	c.Assert(s1.targetInstance.ExternalIp, Equals, Ip("164.77.100.101"))
	c.Assert(*s1.Ssh[0], Equals, "echo \"Host gce-host-0 running qoriolabs/passport:12 build 12\"")
	c.Assert(*s1.Ssh[1], Equals, "docker run -d -p 5050:5050 -v /config:/static/conf:ro --link mongo:mongodb --name passport_12 qoriolabs/passport:12")

	s2 := passport_containers[1]
	c.Assert(s2.targetInstance.ExternalIp, Equals, Ip("164.77.100.102"))
	c.Assert(*s2.Ssh[1], Equals, "docker run -d -p 5050:5050 -v /config:/static/conf:ro --link mongo:mongodb --name passport_12 qoriolabs/passport:12")

}

func (suite *YamlTests) TestGetTasks(c *C) {

	config := &MaestroDoc{}
	c.Log("Reading from", suite.config_file)
	err := config.LoadFromFile(suite.config_file)
	c.Assert(err, Equals, nil)

	err = config.process_config()
	c.Assert(err, Equals, nil)

	context := config.new_context()
	err = config.Validate(context)
	c.Assert(err, Equals, nil)

	// We expect only one task that is the deploy service
	c.Assert(len(config.tasks), Equals, 1)
	config.tasks[0].Run(context)
}

func (suite *YamlTests) TestBuildImages(c *C) {

	config := &MaestroDoc{}
	c.Log("Reading from", suite.images_file)
	err := config.LoadFromFile(suite.images_file)
	c.Assert(err, Equals, nil)

	err = config.process_config()
	c.Assert(err, Equals, nil)

	context := config.new_context()
	err = config.Validate(context)
	c.Assert(err, Equals, nil)

	// We expect only one task that is the deploy service
	c.Assert(len(config.tasks), Equals, 1)
	config.tasks[0].Run(context)
}

func (suite *YamlTests) TestApply(c *C) {

	config := &MaestroDoc{}
	c.Log("Reading from", suite.images_file)
	err := config.LoadFromFile(suite.images_file)
	c.Assert(err, Equals, nil)

	err = config.Apply()
	c.Assert(err, Equals, nil)
}

func (suite *YamlTests) TestApplyPartialConfig(c *C) {
	config := &MaestroDoc{}

	// include the instances and disks yml only
	// keep the images separate.
	config_yml := `
import:
     - ` + suite.instances_file + `
     - ` + suite.disks_file + `

` + yml

	err := config.LoadFromBytes([]byte(config_yml))
	c.Assert(err, Equals, nil)

	c.Assert(err, Equals, nil)

	err = config.Apply()
	c.Assert(err, Equals, nil)
}
