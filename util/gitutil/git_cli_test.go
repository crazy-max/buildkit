package gitutil

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitCLIConfigEnv(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	t.Setenv("USERPROFILE", `C:\Users\tester`)
	t.Setenv("HOMEDRIVE", "C:")
	t.Setenv("HOMEPATH", `\Users\tester`)
	t.Setenv("GIT_CONFIG_GLOBAL", "/tmp/global-gitconfig")
	t.Setenv("GIT_CONFIG_SYSTEM", "/tmp/system-gitconfig")

	t.Run("isolated by default", func(t *testing.T) {
		var got []string
		cli := NewGitCLI(WithExec(func(ctx context.Context, cmd *exec.Cmd) error {
			got = append([]string(nil), cmd.Env...)
			return nil
		}))
		_, err := cli.Run(context.Background(), "status")
		require.NoError(t, err)
		require.Contains(t, got, "GIT_CONFIG_NOSYSTEM=1")
		require.Contains(t, got, "HOME=/dev/null")
		require.NotContains(t, got, "HOME=/tmp/home")
		require.NotContains(t, got, "XDG_CONFIG_HOME=/tmp/xdg")
		require.NotContains(t, got, "GIT_CONFIG_GLOBAL=/tmp/global-gitconfig")
		require.NotContains(t, got, "GIT_CONFIG_SYSTEM=/tmp/system-gitconfig")
	})

	t.Run("host git config opt-in", func(t *testing.T) {
		var got []string
		cli := NewGitCLI(
			WithHostGitConfig(),
			WithExec(func(ctx context.Context, cmd *exec.Cmd) error {
				got = append([]string(nil), cmd.Env...)
				return nil
			}),
		)
		_, err := cli.Run(context.Background(), "status")
		require.NoError(t, err)
		require.NotContains(t, got, "GIT_CONFIG_NOSYSTEM=1")
		require.NotContains(t, got, "HOME=/dev/null")
		require.Contains(t, got, "HOME=/tmp/home")
		require.Contains(t, got, "XDG_CONFIG_HOME=/tmp/xdg")
		require.Contains(t, got, `USERPROFILE=C:\Users\tester`)
		require.Contains(t, got, "HOMEDRIVE=C:")
		require.Contains(t, got, `HOMEPATH=\Users\tester`)
		require.Contains(t, got, "GIT_CONFIG_GLOBAL=/tmp/global-gitconfig")
		require.Contains(t, got, "GIT_CONFIG_SYSTEM=/tmp/system-gitconfig")
	})
}
