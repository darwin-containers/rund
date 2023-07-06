package containerd

import (
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"io"
	"os"
)

type container struct {
	process *os.Process
	request *taskAPI.CreateTaskRequest
	stdin   io.ReadWriteCloser
	stdout  io.ReadWriteCloser
	stderr  io.ReadWriteCloser
}

func (c *container) getPid() int {
	if c.process == nil {
		return 0
	} else {
		return c.process.Pid
	}
}
