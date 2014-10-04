package yaml

import (
	"errors"
	"fmt"
)

func (this *Instance) export_vars() map[string]interface{} {
	vm := make(map[string]map[string]interface{})
	for k, volume := range this.disks {
		vm[string(k)] = map[string]interface{}{
			"mount": volume.MountPoint,
			"disk":  volume.Disk,
		}
	}
	return map[string]interface{}{
		"volumes": vm,
		"name":    this.name,
	}
}

func (this *Image) export_vars() map[string]interface{} {
	artifacts := make(map[string]map[string]interface{})
	for _, a := range this.artifacts {
		artifacts[string(a.name)] = map[string]interface{}{
			"project":  a.Project,
			"source":   a.Source,
			"build":    a.BuildNumber,
			"artifact": a.Artifact,
			"platform": a.Platform,
		}
	}

	return map[string]interface{}{
		"id":         this.RepoId,
		"dockerfile": this.Dockerfile,
		"name":       this.name,
		"artifacts":  artifacts,
	}
}

func (this *Service) Validate(c Context) error {
	// Do variable substitutions
	for _, group := range this.Targets {
		for _, container := range group {
			if container.targetInstance == nil {
				return errors.New(fmt.Sprint("No instance assigned for container", container.name))
			}

			cc := make(Context)
			cc.copy_from(c)
			cc.eval(&container.ImageRef)

			if container.targetImage == nil && container.ImageRef == "" {
				return errors.New(fmt.Sprint("No image for container", container.name))
			}
			if container.targetImage == nil {
				cc["image"] = map[string]interface{}{
					"id": container.ImageRef,
				}
			} else {
				cc["image"] = container.targetImage.export_vars()
			}
			cc["instance"] = container.targetInstance.export_vars()
			for i, ssh := range container.Ssh {
				old := cc.eval(ssh)
				if *container.Ssh[i] == "" {
					return errors.New(fmt.Sprint("Failed to evaluate '", old, "' for container", container.name))
				}
			}
		}
	}

	return nil
}

func (this *Service) InDesiredState(c Context) (bool, error) {
	return true, nil
}

func (this *Service) Prepare(c Context) error {
	return nil
}

func (this *Service) Execute(c Context) error {
	return nil
}

func (this *Service) Finish(c Context) error {
	return nil
}
