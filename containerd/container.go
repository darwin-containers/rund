package containerd

import (
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"sync"
	"time"
)

const unmountFlags = unix.MNT_FORCE

type container struct {
	// These fields are readonly and filled when container is created
	spec          *oci.Spec
	bundlePath    string
	rootfs        string
	dnsSocketPath string
	io            stdio
	console       *os.File

	mu         sync.Mutex
	cmd        *exec.Cmd
	waitblock  chan struct{}
	status     task.Status
	exitStatus uint32
	exitedAt   time.Time
}

func (c *container) destroy() (retErr error) {
	if err := c.io.Close(); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	if c.console != nil {
		if err := c.console.Close(); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}

	// Remove socket file to avoid continuity "failed to create irregular file" error during multiple Dockerfile  `RUN` steps
	_ = os.Remove(c.dnsSocketPath)

	if err := mount.UnmountRecursive(c.rootfs, unmountFlags); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	return
}

func (c *container) setStatusL(status task.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = status
}

func (c *container) getStatusL() task.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.status
}

func (c *container) getConsoleL() *os.File {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.console
}
