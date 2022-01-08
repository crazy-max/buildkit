package client

import (
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

type Vertex struct {
	Digest    digest.Digest
	Inputs    []digest.Digest
	Name      string
	Started   *time.Time
	Completed *time.Time
	Cached    bool
	Error     string
}

type VertexStatus struct {
	ID        string
	Vertex    digest.Digest
	Name      string
	Total     int64
	Current   int64
	Timestamp time.Time
	Started   *time.Time
	Completed *time.Time
}

type VertexLog struct {
	Vertex    digest.Digest
	Stream    int
	Data      []byte
	Timestamp time.Time
}

type VertexWarning struct {
	Vertex     digest.Digest
	Level      int
	Short      []byte
	Detail     [][]byte
	URL        string
	SourceInfo *pb.SourceInfo
	Range      []*pb.Range
}

type SolveStatus struct {
	Vertexes []*Vertex
	Statuses []*VertexStatus
	Logs     []*VertexLog
	Warnings []*VertexWarning
}

type SolveResponse struct {
	ExporterResponse  *controlapi.ExporterResponse
	ExportersResponse []*controlapi.ExporterResponse
}
