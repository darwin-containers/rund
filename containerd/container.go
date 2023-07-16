package containerd

import (
	"github.com/containerd/console"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sys/unix"
	"os/exec"
	"sync"
)

type container struct {
	// These fields are readonly and filled when container is created
	spec       *oci.Spec
	bundlePath string
	rootfs     string
	io         stdio
	console    console.Console

	mu     sync.Mutex
	cmd    *exec.Cmd
	status task.Status
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

	if err := mount.UnmountRecursive(c.rootfs, unix.MNT_FORCE); err != nil {
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

func (c *container) getConsole() console.Console {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.console
}
