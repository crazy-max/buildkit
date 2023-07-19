package export

import (
	"context"

	"github.com/moby/buildkit/session"
	"google.golang.org/grpc"
)

type FinalizeExportCallback func(exporterResponse map[string]string) error

var _ session.Attachable = (*FinalizeExportCallback)(nil)

func (ap FinalizeExportCallback) Register(server *grpc.Server) {
	RegisterExportCallbackServer(server, ap)
}

func (ap FinalizeExportCallback) Finalize(ctx context.Context, req *FinalizeRequest) (*FinalizeResponse, error) {
	return &FinalizeResponse{}, ap(req.ExporterResponse)
}

func Finalize(ctx context.Context, c session.Caller, exporterResponse map[string]string) (bool, error) {
	if !c.Supports(session.MethodURL(_ExportCallback_serviceDesc.ServiceName, "finalize")) {
		return false, nil
	}
	client := NewExportCallbackClient(c.Conn())
	_, err := client.Finalize(ctx, &FinalizeRequest{ExporterResponse: exporterResponse})
	return true, err
}
