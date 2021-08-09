package solver

import (
	"context"
)

type CacheBuildInfos map[interface{}]interface{}

type cacheBuildInfoGetterKey struct{}

func CacheBuildInfoGetterOf(ctx context.Context) func(keys ...interface{}) map[interface{}]interface{} {
	if v := ctx.Value(cacheBuildInfoGetterKey{}); v != nil {
		if getter, ok := v.(func(keys ...interface{}) map[interface{}]interface{}); ok {
			return getter
		}
	}
	return nil
}

func withAncestorCacheBuildInfos(ctx context.Context, start *state) context.Context {
	return context.WithValue(ctx, cacheBuildInfoGetterKey{}, func(keys ...interface{}) map[interface{}]interface{} {
		keySet := make(map[interface{}]struct{})
		for _, k := range keys {
			keySet[k] = struct{}{}
		}
		values := make(map[interface{}]interface{})
		walkAncestors(ctx, start, func(st *state) bool {
			if st.clientVertex.Error != "" {
				// don't use values from cancelled or otherwise error'd vertexes
				return false
			}
			for _, res := range st.op.cacheRes {
				if res.BuildInfos == nil {
					continue
				}
				for k := range keySet {
					if v, ok := res.BuildInfos[k]; ok {
						values[k] = v
						delete(keySet, k)
						if len(keySet) == 0 {
							return true
						}
					}
				}
			}
			return false
		})
		return values
	})
}
