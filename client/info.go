package client

import (
	"context"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/pkg/errors"
)

type InfoResponse struct {
	BuildkitPackage           string
	BuildkitVersion           string
	BuildkitRevision          string
	DockerfileFrontendVersion string
}

func (c *Client) Info(ctx context.Context) (*InfoResponse, error) {
	res, err := c.controlClient().Info(ctx, &controlapi.InfoRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to call info")
	}
	return &InfoResponse{
		BuildkitPackage:           res.BuildkitPackage,
		BuildkitVersion:           res.BuildkitVersion,
		BuildkitRevision:          res.BuildkitRevision,
		DockerfileFrontendVersion: res.DockerfileFrontendVersion,
	}, nil
}
