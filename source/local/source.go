package local

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/contenthash"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/cachedigest"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/patternmatcher"
	"github.com/moby/sys/user"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
	"golang.org/x/time/rate"
)

type Opt struct {
	CacheAccessor cache.Accessor
}

func NewSource(opt Opt) (source.Source, error) {
	ls := &localSource{
		cm: opt.CacheAccessor,
	}
	return ls, nil
}

type localSource struct {
	cm cache.Accessor
}

func (ls *localSource) Schemes() []string {
	return []string{srctypes.LocalScheme}
}

func (ls *localSource) Identifier(scheme, ref string, attrs map[string]string, platform *pb.Platform) (source.Identifier, error) {
	id, err := NewLocalIdentifier(ref)
	if err != nil {
		return nil, err
	}

	for k, v := range attrs {
		switch k {
		case pb.AttrLocalSessionID:
			id.SessionID = v
			if p := strings.SplitN(v, ":", 2); len(p) == 2 {
				id.Name = p[0] + "-" + id.Name
				id.SessionID = p[1]
			}
		case pb.AttrIncludePatterns:
			var patterns []string
			if err := json.Unmarshal([]byte(v), &patterns); err != nil {
				return nil, err
			}
			id.IncludePatterns = patterns
		case pb.AttrExcludePatterns:
			var patterns []string
			if err := json.Unmarshal([]byte(v), &patterns); err != nil {
				return nil, err
			}
			id.ExcludePatterns = patterns
		case pb.AttrFollowPaths:
			var paths []string
			if err := json.Unmarshal([]byte(v), &paths); err != nil {
				return nil, err
			}
			id.FollowPaths = paths
		case pb.AttrSharedKeyHint:
			id.SharedKeyHint = v
		case pb.AttrLocalDiffer:
			switch v {
			case pb.AttrLocalDifferMetadata, "":
				id.Differ = fsutil.DiffMetadata
			case pb.AttrLocalDifferNone:
				id.Differ = fsutil.DiffNone
			}
		case pb.AttrMetadataTransfer:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid value for local.metadatatransfer %q", v)
			}
			id.MetadataOnly = b
		case pb.AttrMetadataTransferExclude:
			var exceptions []string
			if err := json.Unmarshal([]byte(v), &exceptions); err != nil {
				return nil, err
			}
			id.MetadataExceptions = exceptions
		}
	}

	return id, nil
}

func (ls *localSource) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, _ solver.Vertex) (source.SourceInstance, error) {
	localIdentifier, ok := id.(*LocalIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid local identifier %v", id)
	}

	return &localSourceHandler{
		src:         *localIdentifier,
		sm:          sm,
		localSource: ls,
	}, nil
}

type localSourceHandler struct {
	src LocalIdentifier
	sm  *session.Manager
	*localSource
}

func (ls *localSourceHandler) CacheKey(ctx context.Context, g session.Group, index int) (string, string, solver.CacheOpts, bool, error) {
	sessionID := ls.src.SessionID

	if sessionID == "" {
		id := g.SessionIterator().NextSession()
		if id == "" {
			return "", "", nil, false, errors.New("could not access local files without session")
		}
		sessionID = id
	}
	dt, err := json.Marshal(struct {
		SessionID          string
		IncludePatterns    []string
		ExcludePatterns    []string
		FollowPaths        []string
		MetadataTransfer   bool     `json:",omitempty"`
		MetadataExceptions []string `json:",omitempty"`
	}{
		SessionID:          sessionID,
		IncludePatterns:    ls.src.IncludePatterns,
		ExcludePatterns:    ls.src.ExcludePatterns,
		FollowPaths:        ls.src.FollowPaths,
		MetadataTransfer:   ls.src.MetadataOnly,
		MetadataExceptions: ls.src.MetadataExceptions,
	})
	if err != nil {
		return "", "", nil, false, err
	}
	dgst, err := cachedigest.FromBytes(dt, cachedigest.TypeJSON)
	if err != nil {
		return "", "", nil, false, err
	}
	return "session:" + ls.src.Name + ":" + dgst.String(), dgst.String(), nil, true, nil
}

func (ls *localSourceHandler) Snapshot(ctx context.Context, g session.Group) (cache.ImmutableRef, error) {
	sessionID := ls.src.SessionID
	if sessionID == "" {
		return ls.snapshotWithAnySession(ctx, g)
	}

	timeoutCtx, cancel := context.WithCancelCause(ctx)
	timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
	defer func() { cancel(errors.WithStack(context.Canceled)) }()

	caller, err := ls.sm.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return ls.snapshotWithAnySession(ctx, g)
	}

	ref, err := ls.snapshot(ctx, caller)
	if err != nil {
		var serr filesync.InvalidSessionError
		if errors.As(err, &serr) {
			return ls.snapshotWithAnySession(ctx, g)
		}
		return nil, err
	}
	return ref, nil
}

func (ls *localSourceHandler) snapshotWithAnySession(ctx context.Context, g session.Group) (cache.ImmutableRef, error) {
	var ref cache.ImmutableRef
	err := ls.sm.Any(ctx, g, func(ctx context.Context, _ string, c session.Caller) error {
		r, err := ls.snapshot(ctx, c)
		if err != nil {
			return err
		}
		ref = r
		return nil
	})
	return ref, err
}

func (ls *localSourceHandler) snapshot(ctx context.Context, caller session.Caller) (out cache.ImmutableRef, retErr error) {
	metaSfx := ""
	if ls.src.MetadataOnly {
		metaSfx = ":metadata"
	}
	sharedKey := ls.src.Name + ":" + ls.src.SharedKeyHint + ":" + caller.SharedKey() + metaSfx // TODO: replace caller.SharedKey() with source based hint from client(absolute-path+nodeid)

	var mutable cache.MutableRef
	sis, err := searchSharedKey(ctx, ls.cm, sharedKey)
	if err != nil {
		return nil, err
	}
	for _, si := range sis {
		if m, err := ls.cm.GetMutable(ctx, si.ID()); err == nil {
			bklog.G(ctx).Debugf("reusing ref for local: %s", m.ID())
			mutable = m
			break
		} else {
			bklog.G(ctx).Debugf("not reusing ref %s for local: %v", si.ID(), err)
		}
	}

	if mutable == nil {
		m, err := ls.cm.New(ctx, nil, nil, cache.CachePolicyRetain, cache.WithRecordType(client.UsageRecordTypeLocalSource), cache.WithDescription(fmt.Sprintf("local source for %s", ls.src.Name)))
		if err != nil {
			return nil, err
		}
		mutable = m
		bklog.G(ctx).Debugf("new ref for local: %s", mutable.ID())
	}

	defer func() {
		if retErr != nil && mutable != nil {
			// on error remove the record as checksum update is in undefined state
			if err := mutable.SetCachePolicyDefault(); err != nil {
				bklog.G(ctx).Errorf("failed to reset mutable cachepolicy: %v", err)
			}
			contenthash.ClearCacheContext(mutable)
			go mutable.Release(context.WithoutCancel(ctx))
		}
	}()

	mount, err := mutable.Mount(ctx, false, nil)
	if err != nil {
		return nil, err
	}

	lm := snapshot.LocalMounter(mount)

	dest, err := lm.Mount()
	if err != nil {
		return nil, err
	}

	defer func() {
		if retErr != nil && lm != nil {
			lm.Unmount()
		}
	}()

	cc, err := contenthash.GetCacheContext(ctx, mutable)
	if err != nil {
		return nil, err
	}

	opt := filesync.FSSendRequestOpt{
		Name:            ls.src.Name,
		IncludePatterns: ls.src.IncludePatterns,
		ExcludePatterns: ls.src.ExcludePatterns,
		FollowPaths:     ls.src.FollowPaths,
		DestDir:         dest,
		CacheUpdater:    &cacheUpdater{cc, mount.IdentityMapping()},
		ProgressCb:      newProgressHandler(ctx, "transferring "+ls.src.Name+":"),
		Differ:          ls.src.Differ,
		MetadataOnly:    ls.src.MetadataOnly,
	}

	if opt.MetadataOnly && len(ls.src.MetadataExceptions) > 0 {
		matcher, err := patternmatcher.New(ls.src.MetadataExceptions)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		opt.MetadataOnlyFilter = func(p string, _ *fstypes.Stat) bool {
			v, err := matcher.MatchesOrParentMatches(p)
			return err == nil && v
		}
	}

	if idmap := mount.IdentityMapping(); idmap != nil {
		opt.Filter = func(p string, stat *fstypes.Stat) bool {
			uid, gid, err := idmap.ToHost(int(stat.Uid), int(stat.Gid))
			if err != nil {
				return false
			}
			stat.Uid = uint32(uid)
			stat.Gid = uint32(gid)
			return true
		}
	}

	if err := filesync.FSSync(ctx, caller, opt); err != nil {
		return nil, err
	}

	if err := lm.Unmount(); err != nil {
		return nil, err
	}
	lm = nil

	if err := contenthash.SetCacheContext(ctx, mutable, cc); err != nil {
		return nil, err
	}

	// skip storing snapshot by the shared key if it already exists
	md := cacheRefMetadata{mutable}
	if md.getSharedKey() != sharedKey {
		if err := md.setSharedKey(sharedKey); err != nil {
			return nil, err
		}
		bklog.G(ctx).Debugf("saved %s as %s", mutable.ID(), sharedKey)
	}

	snap, err := mutable.Commit(ctx)
	if err != nil {
		return nil, err
	}

	mutable = nil // avoid deferred cleanup

	return snap, nil
}

func newProgressHandler(ctx context.Context, id string) func(int, bool) {
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
		Action:  "transferring",
	}
	pw.Write(id, st)
	return func(s int, last bool) {
		if last || limiter.Allow() {
			st.Current = s
			if last {
				now := time.Now()
				st.Completed = &now
			}
			pw.Write(id, st)
			if last {
				pw.Close()
			}
		}
	}
}

type cacheUpdater struct {
	contenthash.CacheContext
	idmap *user.IdentityMapping
}

func (cu *cacheUpdater) MarkSupported(bool) {
}

func (cu *cacheUpdater) ContentHasher() fsutil.ContentHasher {
	return contenthash.NewFromStat
}

const (
	keySharedKey   = "local.sharedKey"
	sharedKeyIndex = keySharedKey + ":"
)

func searchSharedKey(ctx context.Context, store cache.MetadataStore, k string) ([]cacheRefMetadata, error) {
	var results []cacheRefMetadata
	mds, err := store.Search(ctx, sharedKeyIndex+k, false)
	if err != nil {
		return nil, err
	}
	for _, md := range mds {
		results = append(results, cacheRefMetadata{md})
	}
	return results, nil
}

type cacheRefMetadata struct {
	cache.RefMetadata
}

func (md cacheRefMetadata) getSharedKey() string {
	return md.GetString(keySharedKey)
}

func (md cacheRefMetadata) setSharedKey(key string) error {
	return md.SetString(keySharedKey, key, sharedKeyIndex+key)
}
