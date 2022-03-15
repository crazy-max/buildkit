package release

import (
	_ "embed"
	"testing"

	"github.com/moby/buildkit/version"
	"github.com/stretchr/testify/assert"
)

func TestFrontendVersion(t *testing.T) {
	cases := []struct {
		name  string
		bkver string
		want  string
	}{
		{
			name:  "standard",
			bkver: "v0.10.0",
			want:  "1.4.0",
		},
		{
			name:  "rc",
			bkver: "v0.9.0-rc1",
			want:  "1.3.0-rc1",
		},
		{
			name:  "dirty",
			bkver: "v0.10.0-7-g50735431.m",
			want:  "~1.4.0",
		},
		{
			name:  "dirty",
			bkver: "v0.8.0-6-g7c3e9fdd.m",
			want:  "~1.2.0",
		},
		{
			name:  "default",
			bkver: "0.0.0+unknown",
			want:  "~1.4.0",
		},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			version.Version = tt.bkver
			ver, err := FrontendVersion()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, ver)
		})
	}
}
