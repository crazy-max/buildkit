package exptypes

import (
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ExporterConfigDigestKey      = "config.digest"
	ExporterImageDigestKey       = "containerimage.digest"
	ExporterImageConfigKey       = "containerimage.config"
	ExporterImageConfigDigestKey = "containerimage.config.digest"
	ExporterInlineCache          = "containerimage.inlinecache"
	ExporterBuildInfo            = "containerimage.buildinfo"
	ExporterPlatformsKey         = "refs.platforms"
)

const EmptyGZLayer = digest.Digest("sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1")

type Platforms struct {
	Platforms []Platform
}

type Platform struct {
	ID       string
	Platform ocispecs.Platform
}

type BuildInfo struct {
	Type  BuildInfoType `json:"type,omitempty"`
	Ref   string        `json:"ref,omitempty"`
	Alias string        `json:"alias,omitempty"`
	Pin   string        `json:"pin,omitempty"`
}

type BuildInfoType string

const (
	BuildInfoTypeImage BuildInfoType = "docker-image"
	BuildInfoTypeGit   BuildInfoType = "git"
	BuildInfoTypeHTTP  BuildInfoType = "http"
	BuildInfoTypeHTTPS BuildInfoType = "https"
)
