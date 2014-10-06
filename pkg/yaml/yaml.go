package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"text/template"
)

func (this Context) copy_from(other Context) {
	for k, v := range other {
		this[k] = v
	}
}

func (this Context) eval(f *string) string {
	old := *f
	s := eval(*f, this)
	*f = s
	return old
}

const LIVE_MODE = "LIVE_MODE"

func (this Context) test_mode() bool {
	test := true
	if v, ok := this[LIVE_MODE].(string); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			test = !b
		}
	} else if v, ok := this[LIVE_MODE].(int); ok {
		return v == 0
	}
	return test
}

func eval(tpl string, m map[string]interface{}) string {
	var buff bytes.Buffer
	t := template.Must(template.New(tpl).Parse(tpl))
	err := t.Execute(&buff, m)
	if err != nil {
		return tpl
	} else {
		return os.ExpandEnv(buff.String())
	}
}

func (this *MaestroDoc) String() string {
	bytes, err := yaml.Marshal(this)
	if err != nil {
		return err.Error()
	} else {
		return string(bytes)
	}
}

func (this *MaestroDoc) LoadFromFile(filename string) error {
	this.init()

	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	buff, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	return this.LoadFromBytes(buff)
}

func (this *MaestroDoc) LoadFromBytes(buff []byte) error {
	this.init()

	err := yaml.Unmarshal(buff, this)
	if err != nil {
		return err
	}

	// Do imports
	for _, file := range this.Imports {
		path := os.ExpandEnv(string(file))
		imported := &MaestroDoc{}
		if err := imported.LoadFromFile(path); err != nil {
			return err
		} else {
			imported.merge(this)
		}
	}
	return err
}

func (this *MaestroDoc) init() {
	if this.Imports == nil {
		this.Imports = []YmlFilePath{}
	}
	if this.Deploys == nil {
		this.Deploys = []ServiceKey{}
	}
	if this.Vars == nil {
		this.Vars = make(map[string]string)
	}
	if this.ServiceSection == nil {
		this.ServiceSection = make(map[ServiceKey][]map[JobKey]JobPortList)
	}
	if this.Artifacts == nil {
		this.Artifacts = make(map[ArtifactKey]*Artifact)
	}
	if this.Images == nil {
		this.Images = make(map[ImageKey]*Image)
	}
	if this.Containers == nil {
		this.Containers = make(map[ContainerKey]*Container)
	}
	if this.Disks == nil {
		this.Disks = make(map[DiskKey]*Disk)
	}
	if this.Instances == nil {
		this.Instances = make(map[InstanceKey]*Instance)
	}
	if this.services == nil {
		this.services = make(map[ServiceKey]*Service)
	}
}

func (this *MaestroDoc) merge(other *MaestroDoc) error {
	from := this
	to := other

	for _, d := range from.Deploys {
		to.Deploys = append(to.Deploys, d)
	}
	for k, v := range from.Vars {
		if _, has := to.Vars[k]; has {
			log.Println("Warning: Var[", k, "] to be overriden.")
		}
		to.Vars[k] = v
	}
	for k, v := range from.Artifacts {
		if _, has := to.Artifacts[k]; has {
			log.Println("Warning: Artifact[", k, "] to be overriden.")
		}
		to.Artifacts[k] = v
	}
	for k, v := range from.Images {
		if _, has := to.Images[k]; has {
			log.Println("Warning: Docker[", k, "] to be overriden.")
		}
		to.Images[k] = v
	}
	for k, v := range from.Containers {
		if _, has := to.Containers[k]; has {
			log.Println("Warning: Container[", k, "] to be overriden.")
		}
		to.Containers[k] = v
	}
	for k, v := range from.Disks {
		if _, has := to.Disks[k]; has {
			log.Println("Warning: Disk[", k, "] to be overriden.")
		}
		to.Disks[k] = v
	}
	for k, v := range from.Instances {
		if _, has := to.Instances[k]; has {
			log.Println("Warning: Instance[", k, "] to be overriden.")
		}
		to.Instances[k] = v
	}
	for k, v := range from.Jobs {
		if _, has := to.Jobs[k]; has {
			log.Println("Warning: Job[", k, "] to be overriden.")
		}
		to.Jobs[k] = v
	}
	for k, v := range from.ServiceSection {
		if _, has := to.ServiceSection[k]; has {
			log.Println("Warning: Service[", k, "] to be overriden.")
		}
		to.ServiceSection[k] = v
	}

	return nil
}

func (this *Container) clone_from(other *Container) {
	*this = *other
	// we need clone the ssh commands since they are stored as array of pointers
	ssh := make([]*string, len(other.Ssh))
	for i, c := range other.Ssh {
		copy := *c
		ssh[i] = &copy
	}
	this.Ssh = ssh
}

func (this *MaestroDoc) process_jobs() error {
	for k, job := range this.Jobs {

		container, has := this.Containers[job.ContainerKey]
		if !has {
			return errors.New(fmt.Sprintf("container-%s-not-found", job.ContainerKey))
		}
		job.name = JobKey(k)
		job.container = container
		job.instance_labels = job.InstanceLabels.parse()

		if job.container == nil {
			return errors.New(fmt.Sprintf("job-no-container-template:%s", job.name))
		}

		if len(this.Instances) == 0 {
			return errors.New("no-instances-specified-in-doc")
		}

		// Go through the instances known to this doc and see if an instance can take the job
		matches := []*Instance{}
		for _, instance := range this.Instances {
			if instance.can_take(job) {
				matches = append(matches, instance)
			}
		}
		sort.Sort(ByInstanceKey(matches))
		job.instances = matches

		if len(job.instances) == 0 {
			return errors.New(fmt.Sprintf("job-no-matched-instances:%s", job.name))
		}

		// now create container instances that are to be run on each machine instance
		job.container_instances = []*Container{}
		for _, instance := range job.instances {
			// Spawn a container instance
			copy := &Container{}
			copy.clone_from(job.container)
			copy.targetInstance = instance
			job.container_instances = append(job.container_instances, copy)
		}
		if len(job.container_instances) == 0 {
			return errors.New(fmt.Sprintf("job-no-matched-container-instances:%s", job.name))
		}

	}
	return nil
}

func (this *MaestroDoc) process_services() error {
	// Build the services.  Each service is a composition of a list of container and instance label pair.
	// It means for a given service, there are 1 or more containers to run and they are to be run in sequence.
	// For a given container, it is to be run over a number of instances, as identified by the instance label.
	this.services = make(map[ServiceKey]*Service)
	for service_key, job_key_port_list := range this.ServiceSection {

		service := &Service{
			name:     service_key,
			jobs:     []*Job{},
			port_map: map[JobKey][]ExposedPort{},
		}
		this.services[service_key] = service

		// Here is an array of map (job name : port list) pairs
		for _, job_and_ports := range job_key_port_list {

			for job_key, job_port_list := range job_and_ports {

				// Get the job to determine which containers to run
				job, has := this.Jobs[job_key]

				if !has {
					return errors.New(fmt.Sprintf("job-not-found%s", job_key))
				}

				exposed_ports, err := job_port_list.parse()
				if err != nil {
					return errors.New("bad-port-list:" + err.Error())
				}

				service.jobs = append(service.jobs, job)
				service.port_map[job_key] = exposed_ports
			}
		}
	}
	return nil
}

func (this *MaestroDoc) process_images() error {
	for k, artifact := range this.Artifacts {
		artifact.name = ArtifactKey(k)
	}
	for k, image := range this.Images {
		image.name = k
		for _, ak := range this.Images[k].ArtifactKeys {
			if artifact, has := this.Artifacts[ak]; has {
				if image.artifacts == nil {
					image.artifacts = make([]*Artifact, 0)
				}
				image.artifacts = append(image.artifacts, artifact)
			}
		}
	}
	return nil
}

func (this *MaestroDoc) process_containers() error {
	for k, container := range this.Containers {
		container.name = ContainerKey(k)
		// containers either reference images or builds

		if image, has := this.Images[ImageKey(container.ImageRef)]; has {
			container.targetImage = image
		} else {
			// assumes that the image references a docker hub image
			container.targetImage = nil
		}
		// Containers also reference instances which are bound later at the time
		// when we launch container in instances in parallel.
		// The instances are bound via definition of services.
	}
	return nil
}

func (this *MaestroDoc) process_resources() error {
	for k, a := range this.Disks {
		a.name = DiskKey(k)
	}
	for k, instance := range this.Instances {
		instance.name = InstanceKey(k)
		instance.disks = make(map[VolumeLabel]*Volume)
		instance.labels = instance.InstanceLabels.parse()
		for vl, m := range instance.VolumeSection {
			for dk, mp := range m {
				if _, has := this.Disks[dk]; !has {
					return errors.New(fmt.Sprint("No disk '", dk, "' found."))
				}
				instance.disks[vl] = &Volume{
					Disk:       dk,
					MountPoint: string(mp),
				}
			}
		}
	}

	return nil
}

func (this *MaestroDoc) process_config() error {
	// What to run
	if err := this.process_images(); err != nil {
		return err
	}
	// Where to run things
	if err := this.process_resources(); err != nil {
		return err
	}
	// Containers to run
	if err := this.process_containers(); err != nil {
		return err
	}
	// Jobs to run
	if err := this.process_jobs(); err != nil {
		return err
	}
	// Match what to where:
	if err := this.process_services(); err != nil {
		return err
	}
	return nil
}

func (this *MaestroDoc) new_context() Context {
	context := make(Context)
	for k, v := range this.Vars {
		// Substitutes any $ENV with value from the environment
		context[k] = os.ExpandEnv(v)
	}
	return context
}

func (this *MaestroDoc) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *MaestroDoc) Validate(c Context) error {
	// Validate the doc
	// TODO The resources and artifacts are independent so we can run in parallel

	var err error
	for k, disk := range this.Disks {
		log.Print("Validating disk " + k)
		err = disk.Validate(c)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	for k, instance := range this.Instances {
		log.Print("Validating instance " + k)
		err = instance.Validate(c)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	for k, image := range this.Images {
		log.Print("Validating image " + k)
		err = image.Validate(c)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	log.Println("Image validation done.")
	for k, service := range this.services {
		log.Print("Validating service " + k)
		err = service.Validate(c)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return err
}

func (this *MaestroDoc) all_runnables() []func() Runnable {
	return []func() Runnable{
		this.runnableImages,
	}
}

func (this *MaestroDoc) runnableImages() Runnable {
	m := make(runnableMap)
	for k, v := range this.Images {
		m[k] = v
	}
	return runnableMap(m)
}

func (this *MaestroDoc) runnableServices() Runnable {
	m := make(runnableMap)
	for k, v := range this.services {
		m[k] = v
	}
	return runnableMap(m)
}

func (this *MaestroDoc) runnableDeployments() Runnable {
	m := make(runnableMap)
	for k, v := range this.services {
		m[k] = v
	}
	return runnableMap(m)
}

func (this *MaestroDoc) Prepare(c Context) error {
	for _, prepare := range this.all_runnables() {
		if err := prepare().Prepare(c); err != nil {
			return err
		}
	}
	return nil
}

func (this *MaestroDoc) Execute(c Context) error {
	for _, execute := range this.all_runnables() {
		if err := execute().Execute(c); err != nil {
			return err
		}
	}
	return nil
}

func (this *MaestroDoc) Finish(c Context) error {
	for _, finish := range this.all_runnables() {
		if err := finish().Finish(c); err != nil {
			return err
		}
	}
	return nil
}
