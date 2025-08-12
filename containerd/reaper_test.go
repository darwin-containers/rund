package containerd

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWaitExit(t *testing.T) {
	process, err := os.StartProcess("/bin/sh", []string{"/bin/sh", "-c", "exit 42"}, &os.ProcAttr{})
	require.NoError(t, err)

	w, err := wait(process)
	require.NoError(t, err)
	require.Equal(t, 42, w.ExitCode())
}

func TestWaitKill(t *testing.T) {
	process, err := os.StartProcess("/bin/sh", []string{"/bin/sh", "-c", "sleep 60"}, &os.ProcAttr{})
	require.NoError(t, err)

	err = process.Kill()
	require.NoError(t, err)

	w, err := wait(process)
	require.NoError(t, err)
	require.Equal(t, int(syscall.SIGKILL), int(w.Sys().(syscall.WaitStatus)))
}
