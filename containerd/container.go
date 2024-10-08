package containerd

import (
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sys/unix"
	"os"
	"sync"
)

const unmountFlags = unix.MNT_FORCE

type container struct {
	// These fields are readonly and filled when container is created
	spec          *oci.Spec
	bundlePath    string
	rootfs        string
	dnsSocketPath string

	mu sync.Mutex

	// primary is the primary process for the container.
	// The lifetime of the container is tied to this process.
	primary managedProcess

	// auxiliary is a map of additional processes that run in the jail.
	auxiliary map[string]*managedProcess
}

func (c *container) destroy() (retErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range c.auxiliary {
		if err := p.destroy(); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}

	if err := c.primary.destroy(); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	// Remove socket file to avoid continuity "failed to create irregular file" error during multiple Dockerfile  `RUN` steps
	_ = os.Remove(c.dnsSocketPath)

	if err := mount.UnmountRecursive(c.rootfs, unmountFlags); err != nil {
		retErr = multierror.Append(retErr, err)
	}

	return
}

func (c *container) getProcessL(execID string) (*managedProcess, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getProcess(execID)
}

func (c *container) getProcess(execID string) (*managedProcess, error) {
	if execID == "" {
		return &c.primary, nil
	}

	p := c.auxiliary[execID]

	if p == nil {
		return nil, errgrpc.ToGRPCf(errdefs.ErrNotFound, "exec not found: %s", execID)
	}

	return p, nil
}
