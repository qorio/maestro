package yaml

// Interface to encapsulate build/ deployment behavior of different
// resources and artifacts.

type Context map[string]interface{}
type Runnable interface {
	Validate(c Context) (bool, error)
	InDesiredState(c Context) (bool, error)
	Prepare(c Context) error
	Execute(c Context) error
	Finish(c Context) error
}

type ArtifactKey string
type Artifact struct {
	Project     string `yaml:"project"`
	Source      string `yaml:"source"`
	BuildNumber string `yaml:"build"`
	Artifact    string `yaml:"artifact"`
	Platform    string `yaml:"platform"`

	Name ArtifactKey
}

type ImageKey string
type Image struct {
	Dockerfile   string        `yaml:"dockerfile"`
	RepoId       string        `yaml:"image"`
	ArtifactKeys []ArtifactKey `yaml:"artifacts"`

	Name      ImageKey
	artifacts []*Artifact
}

type ContainerKey string
type Container struct {
	ImageRef string    `yaml:"image"`
	Ssh      []*string `yaml:"ssh"`

	Name           ContainerKey
	TargetInstance *Instance
	TargetImage    *Image
}

type DiskKey string
type Disk struct {
	Cloud string `yaml:"cloud"`
	Type  string `yaml:"disk-type"`
	Size  string `yaml:"size-gb"`

	Name DiskKey
}

type Ip string
type InstanceKey string
type MountPoint string
type InstanceLabel string
type Volume struct {
	Disk       DiskKey
	MountPoint string
}
type VolumeLabel string
type Instance struct {
	Keyfile        string                                 `yaml:"keyfile"`
	Cloud          string                                 `yaml:"cloud"`
	Project        string                                 `yaml:"project"`
	InternalIp     Ip                                     `yaml:"internal-ip"`
	ExternalIp     Ip                                     `yaml:"external-ip"`
	InstanceLabels []InstanceLabel                        `yaml:"labels"`
	VolumeSection  map[VolumeLabel]map[DiskKey]MountPoint `yaml:"volumes"`
	Name           InstanceKey
	Disks          map[VolumeLabel]*Volume
}

type ServiceKey string
type ServiceSection map[ServiceKey][]map[ContainerKey]InstanceLabel
type Service struct {
	Name    ServiceKey
	Targets [][]*Container
	Spec    []map[ContainerKey]InstanceLabel
}
type YmlFilePath string

type MaestroDoc struct {
	Imports        []YmlFilePath                                   `yaml:"import"`
	Deploys        []ServiceKey                                    `yaml:"deploy"`
	Vars           map[string]string                               `yaml:"var"`
	ServiceSection map[ServiceKey][]map[ContainerKey]InstanceLabel `yaml:"service"`
	Artifacts      map[ArtifactKey]*Artifact                       `yaml:"artifact"`
	Images         map[ImageKey]*Image                             `yaml:"image"`
	Containers     map[ContainerKey]*Container                     `yaml:"container"`
	Resources      struct {
		Disks     map[DiskKey]*Disk         `yaml:"disk"`
		Instances map[InstanceKey]*Instance `yaml:"instance"`
	} `yaml:"resource"`

	// Parsed and populated
	Services map[ServiceKey]*Service
}
