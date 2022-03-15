package release

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/moby/buildkit/version"
	"github.com/pkg/errors"
)

//go:embed releases.json
var dtreleases []byte

func FrontendVersion() (string, error) {
	var releases map[string]string
	if err := json.Unmarshal(dtreleases, &releases); err != nil {
		return "", errors.Wrap(err, "failed to parse frontend releases")
	}
	if v, ok := releases[strings.TrimPrefix(version.Version, "v")]; ok {
		return v, nil
	}

	// sort releases
	rkeys := make([]string, 0, len(releases))
	for k := range releases {
		rkeys = append(rkeys, k)
	}
	vs := make([]*semver.Version, len(rkeys))
	for i, r := range rkeys {
		v, err := semver.NewVersion(r)
		if err != nil {
			return "", errors.Wrapf(err, "failed to parse version %s", r)
		}
		vs[i] = v
	}
	sort.Sort(semver.Collection(vs))

	// return latest version if default buildkit version
	if version.Version == version.DefaultVersion {
		return "~" + releases[vs[len(vs)-1].Original()], nil
	}

	// parse buildkit version and returns a semver instance
	// if it's an invalid semantic version, return N/A
	if bksemver, err := semver.NewVersion(version.Version); err == nil {
		// find the closest frontend version
		for _, v := range vs {
			if v.Prerelease() == "" && v.Compare(bksemver) > -1 {
				return "~" + releases[v.Original()], nil
			}
		}
	}

	return "N/A", nil
}
