package yaml

// Interface to encapsulate build/ deployment behavior of different
// resources and artifacts.

type Context map[string]interface{}

type Verifiable interface {
	Validate(c Context) (bool, error)
	InDesiredState(c Context) (bool, error)
}

type Runnable interface {
	Prepare(c Context) error
	Execute(c Context) error
	Finish(c Context) error
}

type ArtifactKey string
type Artifact struct {
	Project        string `yaml:"project"`
	Source         string `yaml:"source"`
	SourceApiToken string `yaml:"source-api-token"`
	BuildNumber    string `yaml:"build"`
	File           string `yaml:"file"`
	Platform       string `yaml:"platform"`

	name ArtifactKey

	task *task
}

type ImageKey string
type Image struct {
	Dockerfile   string        `yaml:"dockerfile"`
	RepoId       string        `yaml:"image"`
	ArtifactKeys []ArtifactKey `yaml:"artifacts"`

	name      ImageKey
	artifacts []*Artifact

	task *task
}

type ContainerKey string
type Container struct {
	ResourceRequirements *Requirement `yaml:"requires"`
	ImageRef             string       `yaml:"image"`
	Ssh                  []*string    `yaml:"ssh"`

	name           ContainerKey
	targetInstance *Instance
	targetImage    *Image

	task *task
}

type SizeQuantityUnit string

const (
	TbFormat SizeQuantityUnit = "%dT"
	GbFormat SizeQuantityUnit = "%dG"
	MbFormat SizeQuantityUnit = "%dM"
	KbFormat SizeQuantityUnit = "%dK"
)

type Resource struct {
	CPU    int              `yaml:"cpu"`
	Memory SizeQuantityUnit `yaml:"memory"`
	Disk   SizeQuantityUnit `yaml:"disk"`
}

type DiskKey string
type Disk struct {
	Cloud string           `yaml:"cloud"`
	Type  string           `yaml:"type"`
	Size  SizeQuantityUnit `yaml:"size"`

	name DiskKey

	task *task
}

type CommaSeparatedList string

type InstanceLabelList CommaSeparatedList
type Ip string
type InstanceKey string
type MountPoint string
type InstanceLabel string
type Volume struct {
	Disk       DiskKey
	MountPoint string

	disk *Disk
	host *Instance
}
type VolumeLabel string
type Instance struct {
	Resource       *Resource                              `yaml:"available"`
	Keypair        string                                 `yaml:"keypair"`
	User           string                                 `yaml:"user"`
	Cloud          string                                 `yaml:"cloud"`
	Project        string                                 `yaml:"project"`
	InternalIp     Ip                                     `yaml:"internal-ip"`
	ExternalIp     Ip                                     `yaml:"external-ip"`
	InstanceLabels InstanceLabelList                      `yaml:"labels"`
	VolumeSection  map[VolumeLabel]map[DiskKey]MountPoint `yaml:"volumes"`

	name   InstanceKey
	disks  map[VolumeLabel]*Volume
	labels []InstanceLabel

	task *task
}

type JobKey string
type JobPortList CommaSeparatedList
type ExposedPort int

// Job - has container, instance labels, and resource requirements
type Job struct {
	ContainerKey   ContainerKey      `yaml:"container"`
	InstanceLabels InstanceLabelList `yaml:"instance-labels"`

	// Global resource requirements
	ResourceRequirements *Requirement `yaml:"requires"`

	name                JobKey
	container           *Container
	instance_labels     []InstanceLabel
	instances           []*Instance
	container_instances []*Container

	task *task
}

type Requirement Resource

type ServiceKey string
type Service struct {
	name     ServiceKey
	jobs     []*Job
	port_map map[JobKey][]ExposedPort

	task *task
}

type YmlFilePath string

type MaestroDoc struct {
	Imports        []YmlFilePath                           `yaml:"import"`
	Deploys        []ServiceKey                            `yaml:"deploy"`
	Vars           map[string]string                       `yaml:"var"`
	ServiceSection map[ServiceKey][]map[JobKey]JobPortList `yaml:"service"`
	Artifacts      map[ArtifactKey]*Artifact               `yaml:"artifact"`
	Images         map[ImageKey]*Image                     `yaml:"image"`
	Containers     map[ContainerKey]*Container             `yaml:"container"`
	Disks          map[DiskKey]*Disk                       `yaml:"disk"`
	Instances      map[InstanceKey]*Instance               `yaml:"instance"`
	Jobs           map[JobKey]*Job                         `yaml:"job"`

	// Parsed and populated
	services map[ServiceKey]*Service
	deploys  []*Service
	tasks    []*task
}

type runnableMap map[interface{}]Runnable
