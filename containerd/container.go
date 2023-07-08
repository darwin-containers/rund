package containerd

import (
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"io"
	"os"
	"sync"
)

type container struct {
	mu      sync.Mutex
	process *os.Process
	request *taskAPI.CreateTaskRequest
	stdin   io.ReadWriteCloser
	stdout  io.ReadWriteCloser
	stderr  io.ReadWriteCloser
}

func (c *container) setProcessL(process *os.Process) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.process = process
}

func (c *container) getProcessL() *os.Process {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.process
}
