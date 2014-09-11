package util

type ArtifactKey string
type Artifact struct {
	Project     string `yaml:"project"`
	Source      string `yaml:"source"`
	BuildNumber int64  `yaml:"build-number"`
	Artifact    string `yaml:"artifact"`
	Platform    string `yaml:"platform"`

	Name ArtifactKey
}

type DockerBuildKey string
type DockerBuild struct {
	Dockerfile           string           `yaml:"dockerfile"`
	DockerHubImageAndTag string           `yaml:"image"`
	ArtifactKeys         []DockerBuildKey `yaml:"artifacts"`

	Name      DockerBuildKey
	Artifacts []*Artifact
}

type DockerContainerKey string
type DockerContainer struct {
	Ssh []string `yaml:"ssh"`

	Instance    *Instance
	DockerBuild DockerBuildKey
	Name        DockerContainerKey
}

type DiskKey string
type Disk struct {
	Cloud string `yaml:"cloud"`
	Type  string `yaml:"disk-type"`
	Size  string `yaml:"size-gb"`

	name DiskKey
}

type Ip string
type InstanceKey string
type MountPoint string
type InstanceLabel string
type Volume map[MountPoint]Disk
type VolumeLabel string
type Instance struct {
	Cloud          string                 `yaml:"cloud"`
	Project        string                 `yaml:"project"`
	InternalIp     Ip                     `yaml:"internal-ip"`
	ExternalIp     Ip                     `yaml:"external-ip"`
	InstanceLabels []InstanceLabel        `yaml:"labels"`
	Volumes        map[VolumeLabel]Volume `yaml:"volumes"`

	Name InstanceKey
}

type ServiceKey string
type ServiceSection map[ServiceKey][]map[DockerContainerKey]InstanceLabel
type YmlFile string

type MaestroDoc struct {
	Import    []YmlFile                                             `yaml:"import"`
	Deploy    []ServiceKey                                          `yaml:"deploy"`
	Var       map[string]string                                     `yaml:"var"`
	Service   map[ServiceKey][]map[DockerContainerKey]InstanceLabel `yaml:"service"`
	Artifact  map[ArtifactKey]Artifact                              `yaml:"artifact"`
	Docker    map[DockerBuildKey]DockerBuild                        `yaml:"image"`
	Container map[DockerContainerKey]DockerContainer                `yaml:"container"`
	Resource  struct {
		Disk     map[DiskKey]Disk         `yaml:"disk"`
		Instance map[InstanceKey]Instance `yaml:"instance"`
	}
}
