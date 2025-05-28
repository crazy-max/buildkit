package types

import (
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	BuildKitBuildType = "https://mobyproject.org/buildkit@v1"
)

type BuildConfig struct {
	Definition    []BuildStep              `json:"llbDefinition,omitempty"`
	DigestMapping map[digest.Digest]string `json:"digestMapping,omitempty"`
}

type BuildStep struct {
	ID            string                  `json:"id,omitempty"`
	Op            *pb.Op                  `json:"op,omitempty"`
	Inputs        []string                `json:"inputs,omitempty"`
	ResourceUsage *resourcestypes.Samples `json:"resourceUsage,omitempty"`
}

type Source struct {
	Locations map[string]*pb.Locations `json:"locations,omitempty"`
	Infos     []SourceInfo             `json:"infos,omitempty"`
}

type SourceInfo struct {
	Filename      string                   `json:"filename,omitempty"`
	Language      string                   `json:"language,omitempty"`
	Data          []byte                   `json:"data,omitempty"`
	Definition    []BuildStep              `json:"llbDefinition,omitempty"`
	DigestMapping map[digest.Digest]string `json:"digestMapping,omitempty"`
}

type ImageSource struct {
	Ref      string
	Platform *ocispecs.Platform
	Digest   digest.Digest
	Local    bool
}

type GitSource struct {
	URL    string
	Commit string
}

type HTTPSource struct {
	URL    string
	Digest digest.Digest
}

type LocalSource struct {
	Name string `json:"name"`
}

type Secret struct {
	ID       string `json:"id"`
	Optional bool   `json:"optional,omitempty"`
}

type SSH struct {
	ID       string `json:"id"`
	Optional bool   `json:"optional,omitempty"`
}

type Sources struct {
	Images []ImageSource
	Git    []GitSource
	HTTP   []HTTPSource
	Local  []LocalSource
}

type ProvenancePredicate interface{}

type ProvenancePredicateSLSA02 struct {
	slsa02.ProvenancePredicate
	Invocation  ProvenanceInvocationSLSA02 `json:"invocation,omitempty"`
	BuildConfig *BuildConfig               `json:"buildConfig,omitempty"`
	Metadata    *ProvenanceMetadataSLSA02  `json:"metadata,omitempty"`
}

type ProvenanceInvocationSLSA02 struct {
	ConfigSource slsa02.ConfigSource `json:"configSource,omitempty"`
	Parameters   Parameters          `json:"parameters,omitempty"`
	Environment  Environment         `json:"environment,omitempty"`
}

type ProvenanceMetadataSLSA02 struct {
	slsa02.ProvenanceMetadata
	BuildKitMetadata BuildKitMetadata `json:"https://mobyproject.org/buildkit@v1#metadata,omitempty"`
	Hermetic         bool             `json:"https://mobyproject.org/buildkit@v1#hermetic,omitempty"`
}

type ProvenancePredicateSLSA1 struct {
	slsa1.ProvenancePredicate
	BuildDefinition ProvenanceBuildDefinitionSLSA1 `json:"buildDefinition,omitempty"`
	RunDetails      ProvenanceRunDetailsSLSA1      `json:"runDetails,omitempty"`
}

type ProvenanceBuildDefinitionSLSA1 struct {
	slsa1.ProvenanceBuildDefinition
	ExternalParameters ProvenanceExternalParametersSLSA1 `json:"externalParameters,omitempty"`
}

type ProvenanceRunDetailsSLSA1 struct {
	slsa1.ProvenanceRunDetails
	Metadata *ProvenanceMetadataSLSA1 `json:"metadata,omitempty"`
}

type ProvenanceExternalParametersSLSA1 struct {
	ConfigSource slsa02.ConfigSource `json:"configSource,omitempty"`
	Parameters   Parameters          `json:"parameters,omitempty"`
	Environment  Environment         `json:"environment,omitempty"`
	BuildConfig  *BuildConfig        `json:"buildConfig,omitempty"`
}

type ProvenanceMetadataSLSA1 struct {
	slsa1.BuildMetadata
	BuildKitMetadata BuildKitMetadata `json:"https://mobyproject.org/buildkit@v1#metadata,omitempty"`
	Hermetic         bool             `json:"https://mobyproject.org/buildkit@v1#hermetic,omitempty"`
	// Since v1 completeness and reproducible are somehow implicit from
	// builder.id, but we still keep them for better accuracy and compatibility
	Completeness BuildKitComplete `json:"https://mobyproject.org/buildkit@v1#completeness,omitempty"`
	Reproducible bool             `json:"https://mobyproject.org/buildkit@v1#reproducible,omitempty"`
}

type Parameters struct {
	Frontend string            `json:"frontend,omitempty"`
	Args     map[string]string `json:"args,omitempty"`
	Secrets  []*Secret         `json:"secrets,omitempty"`
	SSH      []*SSH            `json:"ssh,omitempty"`
	Locals   []*LocalSource    `json:"locals,omitempty"`
	// TODO: select export attributes
	// TODO: frontend inputs
}

type Environment struct {
	Platform string `json:"platform"`
}

type BuildKitMetadata struct {
	VCS      map[string]string                  `json:"vcs,omitempty"`
	Source   *Source                            `json:"source,omitempty"`
	Layers   map[string][][]ocispecs.Descriptor `json:"layers,omitempty"`
	SysUsage []*resourcestypes.SysSample        `json:"sysUsage,omitempty"`
}

type BuildKitComplete struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}
