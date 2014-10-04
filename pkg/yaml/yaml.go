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
		this.ServiceSection = make(map[ServiceKey][]map[ContainerKey]InstanceLabel)
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

func (this *MaestroDoc) build_services() error {
	// Build the services.  Each service is a composition of a list of container and instance label pair.
	// It means for a given service, there are 1 or more containers to run and they are to be run in sequence.
	// For a given container, it is to be run over a number of instances, as identified by the instance label.
	this.services = make(map[ServiceKey]*Service)
	for k, service := range this.ServiceSection {
		serviceObject := &Service{
			Name:    k,
			Targets: make([][]*Container, 0),
			Spec:    service,
		}
		this.services[k] = serviceObject
		// Go through each set of containers and assign the instances to run them.
		// The sets will then be started in sequence. Within each set of containers (per instance),
		// the containers are started in parallel.
		for _, keyLabelMap := range service {
			for containerKey, instanceLabel := range keyLabelMap {
				if container, has := this.Containers[containerKey]; has {
					// Now get the instances for a given label:
					container_instances := make([]*Container, 0)

					instance_keys := make([]string, 0)
					for instance_key, _ := range this.Instances {
						instance_keys = append(instance_keys, string(instance_key))
					}
					sort.Strings(instance_keys)

					for _, instance_key := range instance_keys {
						instance := this.Instances[InstanceKey(instance_key)]
						matched := false
						for _, label := range instance.InstanceLabels {
							if label == instanceLabel {
								matched = true
								break
							}
						}
						if matched {
							copy := &Container{}
							copy.clone_from(container)
							copy.targetInstance = instance
							container_instances = append(container_instances, copy)
						}
					}
					serviceObject.Targets = append(serviceObject.Targets, container_instances)
					if len(serviceObject.Targets) == 0 {
						return errors.New("No instances matched to run container " + string(containerKey))
					}
				} else {
					return errors.New("Unknown container key:" + string(containerKey))
				}
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
	// Match what to where:
	if err := this.build_services(); err != nil {
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

func (this *MaestroDoc) _Deploy() error {
	if err := this.process_config(); err != nil {
		return err
	}

	context := this.new_context()
	if err := this.Validate(context); err != nil {
		return err
	}

	alreadyOk, err := this.InDesiredState(context)
	if err != nil {
		return err
	}

	if !alreadyOk {
		err := this.Prepare(context)
		if err != nil {
			return err
		}
		err = this.Execute(context)
		if err != nil {
			return err
		}
		err = this.Finish(context)
		if err != nil {
			return err
		}
	}
	return nil
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
