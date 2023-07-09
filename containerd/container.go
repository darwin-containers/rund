package containerd

import (
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/oci"
	"io"
	"os"
	"sync"
)

type container struct {
	// These fields are readonly and filled when container is created
	spec    *oci.Spec
	rootfs  string
	request *taskAPI.CreateTaskRequest

	// These fields are readonly and filled when container is started
	stdin  io.ReadWriteCloser
	stdout io.ReadWriteCloser
	stderr io.ReadWriteCloser

	mu      sync.Mutex
	process *os.Process
	status  task.Status
}

func (c *container) setStatusL(process *os.Process, status task.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.process = process
	c.status = status
}

func (c *container) getStatusL() (*os.Process, task.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.process, c.status
}
