package llbsolver

import (
	"context"
	"testing"
	"time"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type sessionCaptureOp struct {
	proxyErrorOp
	sessions []string
}

func (o *sessionCaptureOp) Exec(ctx context.Context, job solver.JobContext, inputs []solver.Result) ([]solver.Result, error) {
	it := job.Session().SessionIterator()
	for id := it.NextSession(); id != ""; id = it.NextSession() {
		o.sessions = append(o.sessions, id)
	}
	return nil, o.err
}

func TestLazyResultRegistersProducerSession(t *testing.T) {
	t.Run("live producer", func(t *testing.T) { testLazyProducerSession(t, false) })
	t.Run("discarded producer", func(t *testing.T) { testLazyProducerSession(t, true) })
}

func testLazyProducerSession(t *testing.T, discard bool) {
	stop := errors.New("stop after observing sessions")
	op := &sessionCaptureOp{proxyErrorOp: proxyErrorOp{err: stop}}
	resolved := 0
	s := solver.NewSolver(solver.SolverOpt{ResolveOpFunc: func(solver.Vertex, solver.Builder) (solver.Op, error) {
		resolved++
		return op, nil
	}})
	defer s.Close()
	producer, err := s.NewJob("producer")
	require.NoError(t, err)
	defer producer.Discard()
	producer.SessionID = "producer-session"
	consumer, err := s.NewJob("consumer")
	require.NoError(t, err)
	defer consumer.Discard()
	consumer.SessionID = "consumer-session"
	prepared := 0
	bridge := func(j *solver.Job) *provenanceBridge {
		return &provenanceBridge{llbBridge: &llbBridge{
			builder: j,
			resolveWorker: func() (worker.Worker, error) {
				prepared++
				return proxyErrorWorker{}, nil
			},
		}}
	}
	def, err := llb.Scratch().File(llb.Mkdir("/shared", 0o755)).Marshal(t.Context())
	require.NoError(t, err)
	res, err := bridge(producer).Solve(t.Context(), frontend.SolveRequest{Definition: def.ToPB()}, producer.SessionID)
	require.NoError(t, err)
	defer res.Ref.Release(context.Background())
	require.Zero(t, prepared, "ordinary solves must remain lazy")
	require.NoError(t, res.Ref.Register(t.Context()))
	require.NoError(t, res.Ref.Register(t.Context()))
	require.Equal(t, 1, prepared, "registration must reuse graph preparation")
	require.Empty(t, op.sessions, "registration must not execute the graph")
	require.Zero(t, resolved, "registration must not resolve operations")
	if discard {
		require.NoError(t, producer.Discard())
	}
	_, err = bridge(consumer).Solve(t.Context(), frontend.SolveRequest{Definition: def.ToPB(), Evaluate: true}, consumer.SessionID)
	require.ErrorIs(t, err, stop)
	if discard {
		require.NotContains(t, op.sessions, producer.SessionID)
	} else {
		require.Contains(t, op.sessions, producer.SessionID)
	}
}

func TestReleaseDuringRegistration(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	resume := make(chan struct{})
	b := &provenanceBridge{llbBridge: &llbBridge{
		resolveWorker: func() (worker.Worker, error) {
			close(started)
			select {
			case <-resume:
				return nil, errors.New("preparation stopped")
			case <-ctx.Done():
				return nil, context.Cause(ctx)
			}
		},
	}}
	rp := newResultProxy(b, frontend.SolveRequest{Definition: &pb.Definition{}})
	registered := make(chan error, 1)
	go func() { registered <- rp.Register(ctx) }()
	<-started
	waitCtx, stopWaiting := context.WithCancel(ctx)
	stopWaiting()
	waited := make(chan error, 1)
	go func() { waited <- rp.Register(waitCtx) }()
	select {
	case err := <-waited:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled registration blocked on graph preparation")
	}
	released := make(chan error, 1)
	go func() { released <- rp.Release(ctx) }()
	select {
	case err := <-released:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Release blocked on graph preparation")
	}
	close(resume)
	require.Error(t, <-registered)
	require.ErrorContains(t, rp.Register(ctx), "released")
}

func TestWrapErrorSourceLocations(t *testing.T) {
	dgst := digest.FromString("vertex")
	sourceWithLocations := func(locs *pb.Locations) *pb.Source {
		return &pb.Source{
			Infos: []*pb.SourceInfo{{Filename: "Dockerfile"}},
			Locations: map[string]*pb.Locations{
				string(dgst): locs,
			},
		}
	}

	for _, tc := range []struct {
		name       string
		source     *pb.Source
		wantSource bool
	}{
		{
			name:       "valid location",
			source:     sourceWithLocations(&pb.Locations{Locations: []*pb.Location{{SourceIndex: 0}}}),
			wantSource: true,
		},
		{
			name: "nil source",
		},
		{
			name:   "missing locations",
			source: &pb.Source{},
		},
		{
			name:   "nil locations",
			source: sourceWithLocations(nil),
		},
		{
			name:   "nil location",
			source: sourceWithLocations(&pb.Locations{Locations: []*pb.Location{nil}}),
		},
		{
			name:   "negative source index",
			source: sourceWithLocations(&pb.Locations{Locations: []*pb.Location{{SourceIndex: -1}}}),
		},
		{
			name:   "source index out of range",
			source: sourceWithLocations(&pb.Locations{Locations: []*pb.Location{{SourceIndex: 1}}}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseErr := errors.New("build failed")
			err := errdefs.WrapVertex(baseErr, dgst)
			rp := &resultProxy{req: frontend.SolveRequest{
				Definition: &pb.Definition{Source: tc.source},
			}}

			wrapped := rp.wrapError(err)
			require.ErrorIs(t, wrapped, baseErr)
			sources := errdefs.Sources(wrapped)
			if tc.wantSource {
				require.Len(t, sources, 1)
				require.Equal(t, "Dockerfile", sources[0].Info.Filename)
			} else {
				require.Empty(t, sources)
			}
		})
	}
}
