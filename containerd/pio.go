package containerd

import (
	"context"
	"github.com/containerd/fifo"
	"io"
	"os"
	"syscall"
)

type stdio struct {
	stdinPath  string
	stdoutPath string
	stderrPath string

	stdin  io.ReadCloser
	stdout io.WriteCloser
	stderr io.WriteCloser
}

func setupIO(ctx context.Context, stdin, stdout, stderr string) (io stdio, _ error) {
	io.stdinPath = stdin
	io.stdoutPath = stdout
	io.stderrPath = stderr

	if _, err := os.Stat(stdin); err == nil {
		io.stdin, err = fifo.OpenFifo(ctx, stdin, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return io, err
		}
	}
	if _, err := os.Stat(stdout); err == nil {
		io.stdout, err = fifo.OpenFifo(ctx, stdout, syscall.O_WRONLY, 0)
		if err != nil {
			return io, err
		}
	}
	if _, err := os.Stat(stderr); err == nil {
		io.stderr, err = fifo.OpenFifo(ctx, stderr, syscall.O_WRONLY, 0)
		if err != nil {
			return io, err
		}
	}
	return io, nil
}

func (s stdio) Close() error {
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	if s.stderr != nil {
		_ = s.stderr.Close()
	}
	return nil
}
