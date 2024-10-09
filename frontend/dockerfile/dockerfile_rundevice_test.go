package dockerfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/continuity/fs/fstest"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/util/testutil/integration"
	"github.com/stretchr/testify/require"
	"github.com/tonistiigi/fsutil"
)

var deviceTests = integration.TestFuncs(
	testDeviceEnv,
	testDeviceRunEnv,
)

func testDeviceEnv(t *testing.T, sb integration.Sandbox) {
	if sb.Rootless() {
		t.SkipNow()
	}

	integration.SkipOnPlatform(t, "windows")
	f := getFrontend(t, sb)

	// FIXME: spec dir only cleans up when sandbox is down, we should set spec dir per test with t.TempDir()
	specDir := sb.CDISpecDir()

	require.NoError(t, os.WriteFile(filepath.Join(specDir, "vendor1-device.yaml"), []byte(`
cdiVersion: "0.3.0"
kind: "vendor1.com/device"
devices:
- name: foo
  containerEdits:
    env:
    - FOO=injected
`), 0600))

	dockerfile := []byte(`
FROM busybox AS base
RUN env|sort | tee foo.env
FROM scratch
COPY --from=base /foo.env /
`)

	dir := integration.Tmpdir(
		t,
		fstest.CreateFile("Dockerfile", dockerfile, 0600),
	)

	c, err := client.New(sb.Context(), sb.Address())
	require.NoError(t, err)
	defer c.Close()

	destDir := t.TempDir()

	_, err = f.Solve(sb.Context(), c, client.SolveOpt{
		FrontendAttrs: map[string]string{
			"device": "vendor1.com/device=foo",
		},
		LocalMounts: map[string]fsutil.FS{
			dockerui.DefaultLocalNameDockerfile: dir,
			dockerui.DefaultLocalNameContext:    dir,
		},
		Exports: []client.ExportEntry{
			{
				Type:      client.ExporterLocal,
				OutputDir: destDir,
			},
		},
	}, nil)
	require.NoError(t, err)

	dt, err := os.ReadFile(filepath.Join(destDir, "foo.env"))
	require.NoError(t, err)
	require.Contains(t, string(dt), `FOO=injected`)
}

func testDeviceRunEnv(t *testing.T, sb integration.Sandbox) {
	if sb.Rootless() {
		t.SkipNow()
	}

	integration.SkipOnPlatform(t, "windows")
	f := getFrontend(t, sb)

	// FIXME: spec dir only cleans up when sandbox is down, we should set spec dir per test with t.TempDir()
	specDir := sb.CDISpecDir()

	require.NoError(t, os.WriteFile(filepath.Join(specDir, "vendor1-device.yaml"), []byte(`
cdiVersion: "0.3.0"
kind: "vendor1.com/device"
devices:
- name: foo
  containerEdits:
    env:
    - FOO=injected
`), 0600))

	dockerfile := []byte(`
FROM busybox AS base
RUN --device=vendor1.com/device=foo env|sort | tee foo.env
FROM scratch
COPY --from=base /foo.env /
`)

	dir := integration.Tmpdir(
		t,
		fstest.CreateFile("Dockerfile", dockerfile, 0600),
	)

	c, err := client.New(sb.Context(), sb.Address())
	require.NoError(t, err)
	defer c.Close()

	destDir := t.TempDir()

	_, err = f.Solve(sb.Context(), c, client.SolveOpt{
		LocalMounts: map[string]fsutil.FS{
			dockerui.DefaultLocalNameDockerfile: dir,
			dockerui.DefaultLocalNameContext:    dir,
		},
		Exports: []client.ExportEntry{
			{
				Type:      client.ExporterLocal,
				OutputDir: destDir,
			},
		},
	}, nil)
	require.NoError(t, err)

	dt, err := os.ReadFile(filepath.Join(destDir, "foo.env"))
	require.NoError(t, err)
	require.Contains(t, string(dt), `FOO=injected`)
}
