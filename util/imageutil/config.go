package imageutil

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/resolver/limited"
	"github.com/moby/buildkit/util/resolver/retryhandler"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ContentCache interface {
	content.Ingester
	content.Provider
	content.Manager
}

var leasesMu sync.Mutex
var leasesF []func(context.Context) error

func CancelCacheLeases() {
	leasesMu.Lock()
	for _, f := range leasesF {
		f(context.TODO())
	}
	leasesF = nil
	leasesMu.Unlock()
}

func AddLease(f func(context.Context) error) {
	leasesMu.Lock()
	leasesF = append(leasesF, f)
	leasesMu.Unlock()
}

// ResolveToNonImageError is returned by the resolver when the ref is mutated by policy to a non-image ref
type ResolveToNonImageError struct {
	Ref     string
	Updated string
}

func (e ResolveToNonImageError) Error() string {
	return fmt.Sprintf("ref mutated by policy to non-image: %s://%s -> %s", srctypes.DockerImageScheme, e.Ref, e.Updated)
}

func Config(ctx context.Context, str string, resolver remotes.Resolver, cache ContentCache, leaseManager leases.Manager, p *ocispecs.Platform) (digest.Digest, []byte, error) {
	var platform platforms.MatchComparer
	if p != nil {
		platform = platforms.Only(*p)
	} else {
		platform = platforms.Default()
	}
	ref, err := reference.Parse(str)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	if leaseManager != nil {
		ctx2, done, err := leaseutil.WithLease(ctx, leaseManager, leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
		ctx = ctx2
		defer func() {
			// this lease is not deleted to allow other components to access manifest/config from cache. It will be deleted after 5 min deadline or on pruning inactive builder
			AddLease(done)
		}()
	}

	desc := ocispecs.Descriptor{
		Digest: ref.Digest(),
	}
	if desc.Digest != "" {
		ra, err := cache.ReaderAt(ctx, desc)
		if err == nil {
			info, err := cache.Info(ctx, desc.Digest)
			if err == nil {
				if ok, err := contentutil.HasSource(info, ref); err == nil && ok {
					desc.Size = ra.Size()
					mt, err := DetectManifestMediaType(ra)
					if err == nil {
						desc.MediaType = mt
					}
				}
			}
		}
	}
	// use resolver if desc is incomplete
	if desc.MediaType == "" {
		_, desc, err = resolver.Resolve(ctx, ref.String())
		if err != nil {
			return "", nil, err
		}
	}

	fetcher, err := resolver.Fetcher(ctx, ref.String())
	if err != nil {
		return "", nil, err
	}

	if desc.MediaType == images.MediaTypeDockerSchema1Manifest {
		errMsg := "support Docker Image manifest version 2, schema 1 has been removed. " +
			"More information at https://docs.docker.com/go/deprecated-image-specs/"
		return "", nil, errors.WithStack(cerrdefs.ErrConflict.WithMessage(errMsg))
	}

	children := childrenConfigHandler(cache, platform)
	children = images.LimitManifests(children, platform, 1)

	dslHandler, err := docker.AppendDistributionSourceLabel(cache, ref.String())
	if err != nil {
		return "", nil, err
	}

	handlers := []images.Handler{
		retryhandler.New(limited.FetchHandler(cache, fetcher, str), func(_ []byte) {}),
		dslHandler,
		children,
	}
	if err := images.Dispatch(ctx, images.Handlers(handlers...), nil, desc); err != nil {
		return "", nil, err
	}
	config, err := images.Config(ctx, cache, desc, platform)
	if err != nil {
		return "", nil, err
	}

	dt, err := content.ReadBlob(ctx, cache, config)
	if err != nil {
		return "", nil, err
	}

	return desc.Digest, dt, nil
}

func childrenConfigHandler(provider content.Provider, platform platforms.MatchComparer) images.HandlerFunc {
	return func(ctx context.Context, desc ocispecs.Descriptor) ([]ocispecs.Descriptor, error) {
		var descs []ocispecs.Descriptor
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			// TODO(stevvooe): We just assume oci manifest, for now. There may be
			// subtle differences from the docker version.
			var manifest ocispecs.Manifest
			if err := json.Unmarshal(p, &manifest); err != nil {
				return nil, err
			}

			descs = append(descs, manifest.Config)
		case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			var index ocispecs.Index
			if err := json.Unmarshal(p, &index); err != nil {
				return nil, err
			}

			if platform != nil {
				for _, d := range index.Manifests {
					if d.Platform == nil || platform.Match(*d.Platform) {
						descs = append(descs, d)
					}
				}
			} else {
				descs = append(descs, index.Manifests...)
			}
		case images.MediaTypeDockerSchema2Config, ocispecs.MediaTypeImageConfig, docker.LegacyConfigMediaType,
			intoto.PayloadType:
			// childless data types.
			return nil, nil
		default:
			return nil, errors.Errorf("encountered unknown type %v; children may not be fetched", desc.MediaType)
		}

		return descs, nil
	}
}

func DetectManifestMediaType(ra content.ReaderAt) (string, error) {
	dt := make([]byte, ra.Size())
	if _, err := ra.ReadAt(dt, 0); err != nil {
		return "", err
	}

	return DetectManifestBlobMediaType(dt)
}

func DetectManifestBlobMediaType(dt []byte) (string, error) {
	var mfst struct {
		MediaType *string         `json:"mediaType"`
		Config    json.RawMessage `json:"config"`
		Manifests json.RawMessage `json:"manifests"`
		Layers    json.RawMessage `json:"layers"`
	}

	if err := json.Unmarshal(dt, &mfst); err != nil {
		return "", err
	}

	mt := images.MediaTypeDockerSchema2ManifestList

	if mfst.Config != nil || mfst.Layers != nil {
		mt = images.MediaTypeDockerSchema2Manifest

		if mfst.Manifests != nil {
			return "", errors.Errorf("invalid ambiguous manifest and manifest list")
		}
	}

	if mfst.MediaType != nil {
		switch *mfst.MediaType {
		case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
			if mt != images.MediaTypeDockerSchema2ManifestList {
				return "", errors.Errorf("mediaType in manifest does not match manifest contents")
			}
			mt = *mfst.MediaType
		case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest:
			if mt != images.MediaTypeDockerSchema2Manifest {
				return "", errors.Errorf("mediaType in manifest does not match manifest contents")
			}
			mt = *mfst.MediaType
		}
	}
	return mt, nil
}
