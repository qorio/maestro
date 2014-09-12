package yaml

import (
	"errors"
	"gopkg.in/yaml.v1"
	"io/ioutil"
	"log"
	"os"
	"sort"
)

func (this *MaestroDoc) LoadFromFile(filename string) error {
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
	err := yaml.Unmarshal(buff, this)
	if err != nil {
		return err
	}

	// Do imports
	for _, file := range this.Imports {
		imported := &MaestroDoc{}
		if err := imported.LoadFromFile(string(file)); err != nil {
			return err
		} else {
			imported.merge(this)
		}
		this = imported
	}
	return err
}

func (this *MaestroDoc) merge(other *MaestroDoc) error {
	for _, d := range other.Deploys {
		this.Deploys = append(this.Deploys, d)
	}
	for k, v := range other.Vars {
		if _, has := this.Vars[k]; has {
			log.Println("Warning: Var[", k, "] to be overriden.")
		}
		this.Vars[k] = v
	}
	for k, v := range other.Services {
		if _, has := this.Services[k]; has {
			log.Println("Warning: Service[", k, "] to be overriden.")
		}
		this.Services[k] = v
	}
	for k, v := range other.Artifacts {
		if _, has := this.Artifacts[k]; has {
			log.Println("Warning: Artifact[", k, "] to be overriden.")
		}
		this.Artifacts[k] = v
	}
	for k, v := range other.Images {
		if _, has := this.Images[k]; has {
			log.Println("Warning: Docker[", k, "] to be overriden.")
		}
		this.Images[k] = v
	}
	for k, v := range other.Containers {
		if _, has := this.Containers[k]; has {
			log.Println("Warning: Container[", k, "] to be overriden.")
		}
		this.Containers[k] = v
	}
	for k, v := range other.Resources.Disks {
		if _, has := this.Resources.Disks[k]; has {
			log.Println("Warning: Disk[", k, "] to be overriden.")
		}
		this.Resources.Disks[k] = v
	}
	for k, v := range other.Resources.Instances {
		if _, has := this.Resources.Instances[k]; has {
			log.Println("Warning: Instance[", k, "] to be overriden.")
		}
		this.Resources.Instances[k] = v
	}

	return nil
}

func (this *MaestroDoc) build_services() error {
	// Build the services.  Each service is a composition of a list of container and instance label pair.
	// It means for a given service, there are 1 or more containers to run and they are to be run in sequence.
	// For a given container, it is to be run over a number of instances, as identified by the instance label.
	this.Services = make(map[ServiceKey]*Service)
	for k, service := range this.ServiceSection {
		serviceObject := &Service{
			Name:    k,
			Targets: make([][]*Container, 0),
			Spec:    service,
		}
		this.Services[k] = serviceObject
		// Go through each set of containers and assign the instances to run them.
		// The sets will then be started in sequence. Within each set of containers (per instance),
		// the containers are started in parallel.
		for _, keyLabelMap := range service {
			for containerKey, instanceLabel := range keyLabelMap {
				if container, has := this.Containers[containerKey]; has {
					// Now get the instances for a given label:
					container_instances := make([]*Container, 0)

					instance_keys := make([]string, 0)
					for instance_key, _ := range this.Resources.Instances {
						instance_keys = append(instance_keys, string(instance_key))
					}
					sort.Strings(instance_keys)

					for _, instance_key := range instance_keys {
						instance := this.Resources.Instances[InstanceKey(instance_key)]
						matched := false
						for _, label := range instance.InstanceLabels {
							if label == instanceLabel {
								matched = true
								break
							}
						}
						if matched {
							copy := Container{}
							copy = *container
							copy.instance = instance
							container_instances = append(container_instances, &copy)
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
		artifact.Name = ArtifactKey(k)
	}
	for k, image := range this.Images {
		image.Name = k
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
		container.Name = ContainerKey(k)
		// containers either reference images or builds

		if image, has := this.Images[ImageKey(container.ImageRef)]; has {
			container.image = image
		} else {
			// assumes that the image references a docker hub image
			container.image = nil
		}
		// containers also reference instances which are bound later at the time
		// when we launch container in instances in parallel.
	}
	return nil
}

func (this *MaestroDoc) process_resources() error {
	for k, a := range this.Resources.Disks {
		a.Name = DiskKey(k)
	}
	for k, a := range this.Resources.Instances {
		a.Name = InstanceKey(k)
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

func (this *MaestroDoc) Deploy() error {
	if err := this.process_config(); err != nil {
		return err
	}

	context := make(Context)
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

func (this *MaestroDoc) Validate(c Context) error {
	return nil
}

func (this *MaestroDoc) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *MaestroDoc) Prepare(c Context) error {
	// Populates the vars in the context so that they are globally accessible
	for k, v := range this.Vars {
		c[k] = v
	}
	// Bring instance data into the scope of a container object
	for k, container := range this.Containers {
		container.Name = k
		// find the instance by k

	}
	return nil
}

func (this *MaestroDoc) Execute(c Context) error {
	return nil
}

func (this *MaestroDoc) Finish(c Context) error {
	return nil
}
