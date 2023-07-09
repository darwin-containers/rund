package containerd

import (
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/oci"
	"os/exec"
	"sync"
)

type container struct {
	// These fields are readonly and filled when container is created
	spec   *oci.Spec
	rootfs string
	io     stdio

	mu     sync.Mutex
	cmd    *exec.Cmd
	status task.Status
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
